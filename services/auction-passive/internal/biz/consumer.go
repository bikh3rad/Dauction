package biz

import (
	"application/internal/entity"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// EventConsumer decodes inbound EventEnvelopes from the bus and applies them to
// the auction use case (CLAUDE.md §2 for auction-passive):
//   - lot.scheduled (VICKREY/UNIQBID only) -> create an OPEN auction;
//   - bids.debited -> reconciliation only (the sync debit is authoritative).
//
// Idempotency is enforced downstream via the inbox (consumed_event), keyed by the
// envelope's subject-scoped idempotency_key.
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
	return []string{SubjectLotScheduled, SubjectBidsDebited}
}

// Handle dispatches a raw EventEnvelope. Unknown subjects are ignored (acked) so
// a shared stream doesn't redeliver events this service does not own.
func (c *EventConsumer) Handle(ctx context.Context, raw []byte) error {
	var env eventEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}

	key := env.IdempotencyKey
	if key == "" {
		key = env.EventID
	}

	switch env.Type {
	case SubjectLotScheduled:
		return c.onLotScheduled(ctx, env.Payload, key)
	case SubjectBidsDebited:
		return c.onBidsDebited(ctx, env.Payload)
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
	// Only the two passive modes are ours; DUTCH (and anything else) is acked and
	// ignored — auction-dutch owns it.
	if !mode.Valid() {
		c.logger.DebugContext(ctx, "ignoring non-passive lot.scheduled", "mode", msg.Mode)

		return nil
	}

	lotID, err := uuid.Parse(msg.LotID)
	if err != nil {
		return fmt.Errorf("%w: lot.scheduled lot_id %q", ErrResourceInvalid, msg.LotID)
	}

	scheduledAt := time.Time{}
	if msg.ScheduledAt != "" {
		t, pErr := time.Parse(time.RFC3339Nano, msg.ScheduledAt)
		if pErr != nil {
			return fmt.Errorf("%w: lot.scheduled scheduled_at %q", ErrResourceInvalid, msg.ScheduledAt)
		}

		scheduledAt = t
	}

	in := LotScheduledInput{
		LotID:        lotID,
		Mode:         mode,
		ScheduledAt:  scheduledAt,
		DurationDays: msg.DurationDays,
		ReserveCents: msg.ReserveCents,
	}

	return c.auction.CreateFromLotScheduled(ctx, in, scopedKey(SubjectLotScheduled, key))
}

// onBidsDebited is reconciliation only: the synchronous bids.Debit call made
// before the bid write is authoritative, so this just logs/verifies (CLAUDE.md
// §5). We never mutate state here.
func (c *EventConsumer) onBidsDebited(ctx context.Context, payload []byte) error {
	var msg bidsDebited
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode bids.debited: %w", err)
	}

	c.logger.DebugContext(ctx, "bids.debited reconciliation",
		"account", msg.AccountID, "key", msg.IdempotencyKey, "amount", msg.Amount.Credits)

	return nil
}

// scopedKey namespaces an inbound idempotency key by subject so keys from
// different producers never collide in this service's inbox.
func scopedKey(subject, key string) string {
	return fmt.Sprintf("%s:%s", subject, key)
}
