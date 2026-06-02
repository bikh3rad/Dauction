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
// the lot use case. It is the consume side of CLAUDE.md §2 for catalog:
// object.listed -> create a DRAFT lot. Idempotency is enforced downstream via the
// inbox (consumed_event), keyed by the envelope's idempotency_key.
type EventConsumer struct {
	logger *slog.Logger
	lot    UsecaseLot
}

// NewEventConsumer constructs the consumer.
func NewEventConsumer(logger *slog.Logger, lot UsecaseLot) *EventConsumer {
	return &EventConsumer{
		logger: logger.With("layer", "EventConsumer"),
		lot:    lot,
	}
}

// Subjects returns the NATS subjects this consumer subscribes to.
func (c *EventConsumer) Subjects() []string {
	return []string{SubjectObjectListed}
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
	case SubjectObjectListed:
		return c.onObjectListed(ctx, env.Payload, key)
	default:
		c.logger.DebugContext(ctx, "ignoring unrelated subject", "type", env.Type)

		return nil
	}
}

func (c *EventConsumer) onObjectListed(ctx context.Context, payload []byte, key string) error {
	var msg objectListed
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode object.listed: %w", err)
	}

	objectID, err := uuid.Parse(msg.ObjectID)
	if err != nil {
		return fmt.Errorf("%w: object.listed object_id %q", ErrResourceInvalid, msg.ObjectID)
	}

	ownerID, err := uuid.Parse(msg.OwnerID)
	if err != nil {
		return fmt.Errorf("%w: object.listed owner_id %q", ErrResourceInvalid, msg.OwnerID)
	}

	mode := entity.AuctionMode(msg.Mode)
	if !mode.Valid() {
		return fmt.Errorf("%w: object.listed mode %q", ErrResourceInvalid, msg.Mode)
	}

	in := ObjectListedInput{
		ObjectID:     objectID,
		OwnerID:      ownerID,
		Mode:         mode,
		ReserveCents: msg.Floor.Cents,
	}

	// Appraised value seeds the lot's appraised_value; fall back to the floor when
	// the producer did not include it.
	if msg.Appraised != nil {
		in.AppraisedValueCents = msg.Appraised.Cents
	} else {
		in.AppraisedValueCents = msg.Floor.Cents
	}

	if mode.Timed() {
		days := durationDaysFromProto(msg.Duration)
		if days == 0 {
			return fmt.Errorf("%w: %s object.listed missing duration", ErrResourceInvalid, mode)
		}

		in.DurationDays = &days
	}

	return c.lot.CreateFromObjectListed(ctx, in, scopedKey(SubjectObjectListed, key))
}

// scopedKey namespaces an inbound idempotency key by subject so keys from
// different producers never collide in this service's inbox.
func scopedKey(subject, key string) string {
	return fmt.Sprintf("%s:%s", subject, key)
}
