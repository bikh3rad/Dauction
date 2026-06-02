package biz

import (
	"application/internal/entity"
	"context"

	"github.com/google/uuid"
)

// UsecaseAccount is the account use case consumed by handlers and event
// consumers. It owns tier elevation (which only ever rises) and the KYC mirror.
type UsecaseAccount interface {
	// Get returns the account, creating a GUEST record on first sight so the
	// gateway always has a read model for an authenticated subject.
	Get(ctx context.Context, id uuid.UUID) (entity.Account, error)
	// GrantVIP elevates an account to VIP (house/admin action). Lowering or a
	// no-op elevation is rejected with ErrResourceInvalid.
	GrantVIP(ctx context.Context, id uuid.UUID) (entity.Account, error)
	// ElevateToMember raises a GUEST to MEMBER on invite.redeemed. Idempotent on
	// idempotencyKey via the inbox; already-MEMBER/VIP accounts are left as-is.
	ElevateToMember(ctx context.Context, id uuid.UUID, idempotencyKey string) error
	// ApproveKyc mirrors kyc.approved onto the account. Idempotent on idempotencyKey.
	ApproveKyc(ctx context.Context, id uuid.UUID, idempotencyKey string) error
}

// RepositoryAccount is the persistence seam (implemented by internal/repo,
// mocked in tests). All tier/kyc mutations that emit an event do so atomically
// with an outbox row via the *Tx methods.
type RepositoryAccount interface {
	// Get returns the account or ErrResourceNotFound.
	Get(ctx context.Context, id uuid.UUID) (entity.Account, error)
	// Upsert ensures a GUEST/PENDING account row exists and returns it.
	EnsureExists(ctx context.Context, id uuid.UUID) (entity.Account, error)

	// SetTierTx writes the new tier and an outbox account.tier_changed row in one
	// transaction. Returns the updated account.
	SetTierTx(ctx context.Context, id uuid.UUID, to entity.Tier, outbox entity.OutboxEvent) (entity.Account, error)
	// SetKycTx writes the new kyc status (no event emitted by identity) and, if
	// provided, marks the consumed event. inboxKey scopes idempotency.
	SetKycTx(ctx context.Context, id uuid.UUID, status entity.KycState, inboxKey string) error

	// MarkConsumed records inboxKey in the inbox if absent and returns true when
	// it was newly inserted (i.e. this event has not been processed before).
	MarkConsumed(ctx context.Context, inboxKey string) (bool, error)
}
