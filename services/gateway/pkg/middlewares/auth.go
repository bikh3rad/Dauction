package middlewares

import (
	"log/slog"
	"net/http"
	"strings"
)

// Trusted downstream identity headers. The gateway is the sole writer of these;
// it strips any client-supplied copy first (never trust inbound identity headers)
// and re-injects the resolved values so backend services can trust them.
const (
	HeaderAccountID   = "X-Account-Id"
	HeaderAccountTier = "X-Account-Tier"
	HeaderKycApproved = "X-Kyc-Approved"
)

// AuthMiddleware resolves the caller's account id from the Authorization header
// and injects it as the trusted X-Account-Id header for downstream services.
//
// Dev scheme (documented): `Authorization: Bearer <accountId>` — the bearer token
// IS the account UUID. Real JWT signature validation is future work; the seam is
// here (extractAccountID) so it can be swapped without touching the chain.
//
// Security: inbound X-Account-Id / X-Account-Tier / X-Kyc-Approved are always
// stripped before resolution so a client can never forge its identity.
type AuthMiddleware struct {
	logger *slog.Logger
}

// NewAuthMiddleware constructs the auth middleware.
func NewAuthMiddleware(logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{logger: logger.With("layer", "AuthMiddleware")}
}

// Middleware strips spoofable identity headers, resolves the caller, and (when
// authenticated) injects the trusted X-Account-Id header. Authorization is left
// to the downstream guard; unauthenticated requests pass through so public routes
// still work — the guard rejects them if the matched route requires auth.
func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Never trust inbound identity headers.
		req.Header.Del(HeaderAccountID)
		req.Header.Del(HeaderAccountTier)
		req.Header.Del(HeaderKycApproved)

		if accountID := extractAccountID(req); accountID != "" {
			req.Header.Set(HeaderAccountID, accountID)
		}

		next.ServeHTTP(w, req)
	})
}

// extractAccountID parses the dev bearer scheme: `Authorization: Bearer <accountId>`.
// Returns "" when absent/malformed (caller treated as anonymous).
func extractAccountID(req *http.Request) string {
	authz := req.Header.Get("Authorization")
	if authz == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(authz, prefix))
}
