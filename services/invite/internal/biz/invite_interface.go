package biz

import (
	"application/internal/entity"
	"context"
)

// RedeemResult is the outcome of a successful redemption: the issuer (chain
// parent) is needed to build the invite.redeemed event payload.
type RedeemResult struct {
	Code            string
	IssuerAccountID string
	RedeemedBy      string
}

// ListInvitesFilter narrows an admin invite listing.
type ListInvitesFilter struct {
	Status          string // optional InviteStatus; empty = all
	IssuerAccountID string // optional issuer filter
	Limit           int
	Offset          int
}

// UsecaseInvite is the invite service use-case surface consumed by handlers.
type UsecaseInvite interface {
	// Issue creates a new single-use code owned by issuerAccountID, enforcing the
	// per-issuer quota. Returns ErrResourceInvalid when the quota is exceeded.
	Issue(ctx context.Context, issuerAccountID string) (entity.Invite, error)
	// Redeem atomically consumes an ISSUED code for redeemerAccountID, records the
	// chain edge and an outbox invite.redeemed event. Reused/revoked/flagged/missing
	// codes -> ErrResourceInvalid.
	Redeem(ctx context.Context, code, redeemerAccountID string) (RedeemResult, error)
	// List returns invites for the admin console.
	List(ctx context.Context, f ListInvitesFilter) ([]entity.Invite, error)
	// Revoke marks an ISSUED code REVOKED (admin). Non-ISSUED -> ErrResourceInvalid.
	Revoke(ctx context.Context, code string) error
	// Flag marks an ISSUED code FLAGGED (admin). Non-ISSUED -> ErrResourceInvalid.
	Flag(ctx context.Context, code string) error
	// Chain returns the redeemed invite edges where accountID is the inviter,
	// i.e. the accounts this account brought in.
	Chain(ctx context.Context, accountID string) ([]entity.InviteEdge, error)
}

// RepositoryInvite is the persistence seam implemented by internal/repo and
// mocked in biz tests.
type RepositoryInvite interface {
	// Create inserts a new ISSUED invite.
	Create(ctx context.Context, inv entity.Invite) (entity.Invite, error)
	// CountByIssuer counts all invites issued by an account (quota check).
	CountByIssuer(ctx context.Context, issuerAccountID string) (int, error)
	// Redeem performs the single-use redemption transactionally: a conditional
	// UPDATE ... WHERE status='ISSUED', the invite_edge insert and the outbox row
	// all commit together. Returns ErrResourceInvalid if no ISSUED row matched.
	Redeem(ctx context.Context, code, redeemerAccountID, outboxPayload, idempotencyKey string) (RedeemResult, error)
	// List returns invites matching the filter.
	List(ctx context.Context, f ListInvitesFilter) ([]entity.Invite, error)
	// SetStatus moves an ISSUED code to the target status (conditional UPDATE).
	// Returns ErrResourceInvalid when no ISSUED row matched, ErrResourceNotFound
	// when the code does not exist at all.
	SetStatus(ctx context.Context, code string, target entity.InviteStatus) error
	// Chain returns invite_edge rows where accountID is the inviter.
	Chain(ctx context.Context, accountID string) ([]entity.InviteEdge, error)
}
