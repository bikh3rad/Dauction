package middlewares

import (
	"application/internal/service/dto"
	"application/pkg/utils"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RateLimiter is a fixed-window in-memory rate limiter keyed by client identity
// (authenticated account id when present, else client IP). It is intentionally
// simple and process-local; a multi-replica deployment would back this with a
// shared store, but it satisfies the gateway's per-instance abuse guard.
type RateLimiter struct {
	logger *slog.Logger

	limit  int
	window time.Duration

	mu      sync.Mutex
	windows map[string]*windowCounter

	now func() time.Time
}

type windowCounter struct {
	count int
	reset time.Time
}

// NewRateLimiter builds a limiter allowing `limit` requests per `window`.
func NewRateLimiter(logger *slog.Logger, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		logger:  logger.With("layer", "RateLimiter"),
		limit:   limit,
		window:  window,
		windows: make(map[string]*windowCounter),
		now:     time.Now,
	}
}

// allow records a hit for key and reports whether it is within the window limit.
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()

	wc, ok := rl.windows[key]
	if !ok || now.After(wc.reset) {
		rl.windows[key] = &windowCounter{count: 1, reset: now.Add(rl.window)}

		return true
	}

	if wc.count >= rl.limit {
		return false
	}

	wc.count++

	return true
}

// Middleware enforces the limit. The key prefers the trusted X-Account-Id header
// (set by the auth middleware, which must run first) and falls back to client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		key := req.Header.Get(HeaderAccountID)
		if key == "" {
			key = utils.GetUserIPAddress(req)
		}

		if !rl.allow(key) {
			rl.logger.WarnContext(req.Context(), "rate limit exceeded", "key", key)
			w.Header().Set("Retry-After", "1")
			dto.HandleErrorCode(dto.CodeRateLimited, w)

			return
		}

		next.ServeHTTP(w, req)
	})
}
