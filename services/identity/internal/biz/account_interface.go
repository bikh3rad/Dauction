package biz

import (
	"application/internal/entity"
	"context"

	"github.com/google/uuid"
)

// UserFilter parameterizes the admin user listing (all fields optional).
type UserFilter struct {
	Status string
	Role   string
	Query  string
	Limit  int
	Offset int
}

// UserPatch carries optional admin profile edits; nil fields are left unchanged.
type UserPatch struct {
	Handle *string
	Status *entity.Status
}

// UsecaseAccount is the account use case consumed by handlers and event
// consumers. It owns tier elevation (which only ever rises), the KYC mirror, and
// RBAC role grants + admin user management.
type UsecaseAccount interface {
	// Get returns the account, creating a GUEST record on first sight so the
	// gateway always has a read model for an authenticated subject.
	Get(ctx context.Context, id uuid.UUID) (entity.Account, error)
	// GrantVIP elevates an account to VIP (house/admin action). Lowering or a
	// no-op elevation is rejected with ErrResourceInvalid.
	GrantVIP(ctx context.Context, id uuid.UUID) (entity.Account, error)
	// ElevateToMember raises a GUEST to MEMBER once KYC is approved (invites
	// removed). Idempotent on idempotencyKey via the inbox; already-MEMBER/VIP
	// accounts are left as-is.
	ElevateToMember(ctx context.Context, id uuid.UUID, idempotencyKey string) error
	// ApproveKyc mirrors kyc.approved onto the account. Idempotent on idempotencyKey.
	ApproveKyc(ctx context.Context, id uuid.UUID, idempotencyKey string) error

	// --- RBAC + admin user management ---
	// GrantRole assigns a functional role (e.g. promote to INSPECTOR) and emits
	// account.role_changed. grantedBy is the acting admin (uuid.Nil if unknown).
	GrantRole(ctx context.Context, id uuid.UUID, role entity.Role, grantedBy uuid.UUID) (entity.Account, error)
	// RevokeRole removes a functional role and emits account.role_changed.
	RevokeRole(ctx context.Context, id uuid.UUID, role entity.Role, by uuid.UUID) (entity.Account, error)
	// ListUsers returns admin-filtered accounts + the total match count.
	ListUsers(ctx context.Context, f UserFilter) ([]entity.Account, int, error)
	// UpdateUser applies admin profile edits (handle/status).
	UpdateUser(ctx context.Context, id uuid.UUID, p UserPatch) (entity.Account, error)
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

	// GrantRoleTx inserts an account_role row + the account.role_changed outbox
	// event atomically and returns the refreshed account.
	GrantRoleTx(ctx context.Context, id uuid.UUID, role entity.Role, grantedBy uuid.UUID, outbox entity.OutboxEvent) (entity.Account, error)
	// RevokeRoleTx removes a role + emits account.role_changed atomically.
	RevokeRoleTx(ctx context.Context, id uuid.UUID, role entity.Role, outbox entity.OutboxEvent) (entity.Account, error)
	// ListUsers returns admin-filtered accounts + the total match count.
	ListUsers(ctx context.Context, f UserFilter) ([]entity.Account, int, error)
	// UpdateUser applies admin profile edits and returns the refreshed account.
	UpdateUser(ctx context.Context, id uuid.UUID, p UserPatch) (entity.Account, error)
}
