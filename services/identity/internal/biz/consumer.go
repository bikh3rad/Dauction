package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// kycApproved mirrors dauction.events.v1.KycApproved.
type kycApproved struct {
	AccountID    string `json:"account_id"`
	SubmissionID string `json:"submission_id"`
}

// EventConsumer decodes inbound EventEnvelopes from the bus and applies them to
// the account use case. It is the consume side of CLAUDE.md §2 for identity:
// the invite system was removed, so kyc.approved is now the sole membership
// trigger — it both marks the account KYC-eligible AND elevates GUEST->MEMBER.
// Idempotency is enforced downstream via the inbox (consumed_event), keyed by
// the envelope's idempotency_key.
type EventConsumer struct {
	logger  *slog.Logger
	account UsecaseAccount
}

// NewEventConsumer constructs the consumer.
func NewEventConsumer(logger *slog.Logger, account UsecaseAccount) *EventConsumer {
	return &EventConsumer{
		logger:  logger.With("layer", "EventConsumer"),
		account: account,
	}
}

// Subjects returns the NATS subjects this consumer subscribes to.
func (c *EventConsumer) Subjects() []string {
	return []string{SubjectKycApproved}
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
	case SubjectKycApproved:
		return c.onKycApproved(ctx, env.Payload, key)
	default:
		c.logger.DebugContext(ctx, "ignoring unrelated subject", "type", env.Type)

		return nil
	}
}

func (c *EventConsumer) onKycApproved(ctx context.Context, payload []byte, key string) error {
	var msg kycApproved
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode kyc.approved: %w", err)
	}

	id, err := uuid.Parse(msg.AccountID)
	if err != nil {
		return fmt.Errorf("%w: kyc.approved account_id %q", ErrResourceInvalid, msg.AccountID)
	}

	// KYC is now the membership trigger (invites removed). Mirror the KYC status,
	// then elevate GUEST->MEMBER. Both are idempotent; the elevation uses a
	// distinct scoped key so its inbox row never collides with the KYC mirror.
	if err := c.account.ApproveKyc(ctx, id, scopedKey(SubjectKycApproved, key)); err != nil {
		return err
	}

	return c.account.ElevateToMember(ctx, id, scopedKey(SubjectKycApproved+":member", key))
}

// scopedKey namespaces an inbound idempotency key by subject so keys from
// different producers never collide in this service's inbox.
func scopedKey(subject, key string) string {
	return fmt.Sprintf("%s:%s", subject, key)
}
