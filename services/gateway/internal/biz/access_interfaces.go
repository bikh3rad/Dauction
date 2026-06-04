package biz

import (
	"application/internal/entity"
	"context"
)

// RouteRequirement describes what a matched route demands of the caller. The
// router resolves this per request; the access use case enforces it.
type RouteRequirement struct {
	// Public routes skip auth and the tier/KYC guard entirely (gallery, lot
	// reads, invite redeem, kyc start/verify).
	Public bool
	// RequireMember demands tier ∈ {MEMBER, VIP}.
	RequireMember bool
	// RequireKyc demands kyc_status == APPROVED.
	RequireKyc bool
}

// GuardResult is the outcome of an authorization decision. On Allowed it carries
// the resolved access projection so the proxy can inject trusted downstream
// headers (X-Account-Tier, X-Kyc-Approved).
type GuardResult struct {
	Allowed bool
	Access  entity.Access
}

// UsecaseAccess is consumed by the gateway proxy handler / guard middleware.
type UsecaseAccess interface {
	// Lookup fetches (and briefly caches) the access read model for an account.
	Lookup(ctx context.Context, accountID string) (entity.Access, error)
	// Authorize applies a route's requirements to a caller. accountID may be
	// empty for unauthenticated callers; public routes are allowed regardless.
	Authorize(ctx context.Context, accountID string, req RouteRequirement) (GuardResult, error)
}

// RepositoryAccess is the seam to identity's read model over HTTP. Implemented
// by repo and mocked in biz tests. The gateway never touches identity's DB.
type RepositoryAccess interface {
	// FetchAccess calls identity GET /apis/internal/accounts/{id}/access.
	FetchAccess(ctx context.Context, accountID string) (entity.Access, error)
}
