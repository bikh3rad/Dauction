package biz

import (
	"application/internal/entity"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// EventConsumer decodes inbound EventEnvelopes from the bus and applies them to
// the auction use case (root CLAUDE.md §2 consume side for auction-dutch):
// lot.scheduled (DUTCH only) -> create a SCHEDULED auction; escrow.locked -> flip
// the matching reservation LOCKED. Idempotency is enforced downstream via the
// inbox (consumed_event), keyed by a subject-scoped idempotency_key.
type EventConsumer struct {
	logger  *slog.Logger
	auction UsecaseAuction
}

// NewEventConsumer constructs the consumer.
func NewEventConsumer(logger *slog.Logger, auction UsecaseAuction) *EventConsumer {
	return &EventConsumer{
		logger:  logger.With("layer", "EventConsumer"),
		auction: auction,
	}
}

// Subjects returns the NATS subjects this consumer subscribes to.
func (c *EventConsumer) Subjects() []string {
	return []string{SubjectLotScheduled, SubjectEscrowLocked}
}

// Handle dispatches a raw EventEnvelope. Unknown subjects are ignored (acked) so
// a shared stream doesn't redeliver events this service does not own.
func (c *EventConsumer) Handle(ctx context.Context, raw []byte) error {
	var env eventEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}

	// Fall back to the envelope event_id when no idempotency_key is set, so the
	// inbox always has a stable dedup key.
	key := env.IdempotencyKey
	if key == "" {
		key = env.EventID
	}

	switch env.Type {
	case SubjectLotScheduled:
		return c.onLotScheduled(ctx, env.Payload, key)
	case SubjectEscrowLocked:
		return c.onEscrowLocked(ctx, env.Payload, key)
	default:
		c.logger.DebugContext(ctx, "ignoring unrelated subject", "type", env.Type)

		return nil
	}
}

func (c *EventConsumer) onLotScheduled(ctx context.Context, payload []byte, key string) error {
	var msg lotScheduled
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode lot.scheduled: %w", err)
	}

	mode := entity.AuctionMode(msg.Mode)

	// Non-DUTCH lots are owned by auction-passive; the use case no-ops them, but we
	// still need a valid lot id for DUTCH ones.
	lotID, err := uuid.Parse(msg.LotID)
	if err != nil {
		return fmt.Errorf("%w: lot.scheduled lot_id %q", ErrResourceInvalid, msg.LotID)
	}

	in := LotScheduledInput{
		LotID:        lotID,
		Mode:         mode,
		ReserveCents: msg.ReserveCents,
	}

	// The catalog-supplied auction_id (when present) becomes this auction's id so
	// downstream correlation is stable; otherwise the use case mints one.
	if msg.AuctionID != "" {
		if id, perr := uuid.Parse(msg.AuctionID); perr == nil {
			in.AuctionID = id
		}
	}

	return c.auction.CreateFromLotScheduled(ctx, in, scopedKey(SubjectLotScheduled, key))
}

func (c *EventConsumer) onEscrowLocked(ctx context.Context, payload []byte, key string) error {
	var msg escrowLocked
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode escrow.locked: %w", err)
	}

	in := EscrowLockedInput{
		EscrowRef:   msg.EscrowRef,
		TradeID:     msg.TradeID,
		State:       msg.State,
		AmountCents: msg.Amount.Cents,
	}

	if msg.ParticipantID != "" {
		if pid, err := uuid.Parse(msg.ParticipantID); err == nil {
			in.ParticipantID = pid
		}
	}

	return c.auction.ApplyEscrowLocked(ctx, in, scopedKey(SubjectEscrowLocked, key))
}

// scopedKey namespaces an inbound idempotency key by subject so keys from
// different producers never collide in this service's inbox.
func scopedKey(subject, key string) string {
	return fmt.Sprintf("%s:%s", subject, key)
}
