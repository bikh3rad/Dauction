package biz

import (
	"application/internal/entity"
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// accessCacheTTL is how long a fetched access projection is trusted before the
// gateway re-reads it from identity. Short by design: tier/KYC changes must
// propagate quickly, but every gated request should not hammer identity.
const accessCacheTTL = 5 * time.Second

type cachedAccess struct {
	access entity.Access
	expiry time.Time
}

// access is the gateway's authorization use case. It resolves the tier/KYC read
// model from identity (via the RepositoryAccess seam), caches it briefly, and
// renders the allow/deny decision for a route's requirements.
type access struct {
	logger *slog.Logger
	tracer trace.Tracer
	repo   RepositoryAccess

	ttl   time.Duration
	mu    sync.Mutex
	cache map[string]cachedAccess
	now   func() time.Time
}

var _ UsecaseAccess = (*access)(nil)

// NewAccess constructs the access use case with the default cache TTL.
func NewAccess(logger *slog.Logger, repo RepositoryAccess) *access {
	return &access{
		logger: logger.With("layer", "AccessUsecase"),
		tracer: otel.Tracer("AccessUsecase"),
		repo:   repo,
		ttl:    accessCacheTTL,
		cache:  make(map[string]cachedAccess),
		now:    time.Now,
	}
}

// Lookup returns the access projection for an account, serving from the in-memory
// cache when a fresh entry exists, otherwise reading through to identity.
func (uc *access) Lookup(ctx context.Context, accountID string) (entity.Access, error) {
	ctx, span := uc.tracer.Start(ctx, "Lookup")
	defer span.End()

	if a, ok := uc.fromCache(accountID); ok {
		return a, nil
	}

	a, err := uc.repo.FetchAccess(ctx, accountID)
	if err != nil {
		uc.logger.ErrorContext(ctx, "fetch access failed", "account", accountID, "error", err)

		return entity.Access{}, err
	}

	uc.store(accountID, a)

	return a, nil
}

// Authorize applies the route requirement to the caller. Public routes are always
// allowed (no account needed). Gated routes require an authenticated account that
// satisfies the tier and KYC predicates; the precise sentinel (ErrTierRequired /
// ErrKycRequired / ErrResourceAccessDenied) is returned so the responder can emit
// the right language-neutral code.
func (uc *access) Authorize(
	ctx context.Context, accountID string, req RouteRequirement,
) (GuardResult, error) {
	ctx, span := uc.tracer.Start(ctx, "Authorize")
	defer span.End()

	if req.Public {
		return GuardResult{Allowed: true}, nil
	}

	// A non-public route always needs an authenticated caller.
	if accountID == "" {
		return GuardResult{}, ErrResourceAccessDenied
	}

	a, err := uc.Lookup(ctx, accountID)
	if err != nil {
		return GuardResult{}, err
	}

	if req.RequireMember && !a.IsMember() {
		return GuardResult{}, ErrTierRequired
	}

	if req.RequireKyc && !a.KycApproved() {
		return GuardResult{}, ErrKycRequired
	}

	// Admin route group: dev Basic-Auth (admin/admin) is enforced upstream in the
	// admin middleware; here we also accept an ADMIN-role session (the prod path).
	if req.RequireAdmin && !a.HasRole("ADMIN") {
		return GuardResult{}, ErrAdminRequired
	}

	if req.RequireRole != "" && !a.HasRole(req.RequireRole) {
		return GuardResult{}, ErrRoleRequired
	}

	return GuardResult{Allowed: true, Access: a}, nil
}

func (uc *access) fromCache(accountID string) (entity.Access, bool) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	entry, ok := uc.cache[accountID]
	if !ok || uc.now().After(entry.expiry) {
		return entity.Access{}, false
	}

	return entry.access, true
}

func (uc *access) store(accountID string, a entity.Access) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	uc.cache[accountID] = cachedAccess{
		access: a,
		expiry: uc.now().Add(uc.ttl),
	}
}
