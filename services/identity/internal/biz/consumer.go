package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// inviteRedeemed mirrors dauction.events.v1.InviteRedeemed.
type inviteRedeemed struct {
	Code       string `json:"code"`
	RedeemedBy string `json:"redeemed_by"`
	IssuedBy   string `json:"issued_by"`
}

// kycApproved mirrors dauction.events.v1.KycApproved.
type kycApproved struct {
	AccountID    string `json:"account_id"`
	SubmissionID string `json:"submission_id"`
}

// EventConsumer decodes inbound EventEnvelopes from the bus and applies them to
// the account use case. It is the consume side of CLAUDE.md §2 for identity:
// invite.redeemed -> elevate GUEST->MEMBER; kyc.approved -> mark eligible.
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
	return []string{SubjectInviteRedeemed, SubjectKycApproved}
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
	case SubjectInviteRedeemed:
		return c.onInviteRedeemed(ctx, env.Payload, key)
	case SubjectKycApproved:
		return c.onKycApproved(ctx, env.Payload, key)
	default:
		c.logger.DebugContext(ctx, "ignoring unrelated subject", "type", env.Type)

		return nil
	}
}

func (c *EventConsumer) onInviteRedeemed(ctx context.Context, payload []byte, key string) error {
	var msg inviteRedeemed
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode invite.redeemed: %w", err)
	}

	id, err := uuid.Parse(msg.RedeemedBy)
	if err != nil {
		return fmt.Errorf("%w: invite.redeemed redeemed_by %q", ErrResourceInvalid, msg.RedeemedBy)
	}

	return c.account.ElevateToMember(ctx, id, scopedKey(SubjectInviteRedeemed, key))
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

	return c.account.ApproveKyc(ctx, id, scopedKey(SubjectKycApproved, key))
}

// scopedKey namespaces an inbound idempotency key by subject so keys from
// different producers never collide in this service's inbox.
func scopedKey(subject, key string) string {
	return fmt.Sprintf("%s:%s", subject, key)
}
