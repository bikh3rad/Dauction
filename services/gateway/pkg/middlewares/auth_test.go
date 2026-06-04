package middlewares_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"application/pkg/middlewares"

	"github.com/stretchr/testify/require"
)

// TestAuthMiddleware asserts the dev bearer scheme resolves the account id and,
// critically, that any client-supplied identity headers are stripped before the
// gateway injects its own trusted value.
func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		authz         string
		spoofedHeader string
		wantAccountID string
	}{
		{"bearer resolves account", "Bearer acc-123", "", "acc-123"},
		{"spoofed X-Account-Id stripped (anonymous)", "", "evil", ""},
		{"spoofed X-Account-Id overridden by bearer", "Bearer real", "evil", "real"},
		{"non-bearer ignored", "Basic abc", "", ""},
		{"empty authz anonymous", "", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotAccountID string
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotAccountID = r.Header.Get(middlewares.HeaderAccountID)
			})

			mw := middlewares.NewAuthMiddleware(slog.Default())
			h := mw.Middleware(next)

			req := httptest.NewRequest(http.MethodGet, "/apis/me", nil)
			if tc.authz != "" {
				req.Header.Set("Authorization", tc.authz)
			}
			if tc.spoofedHeader != "" {
				req.Header.Set(middlewares.HeaderAccountID, tc.spoofedHeader)
			}

			h.ServeHTTP(httptest.NewRecorder(), req)

			require.Equal(t, tc.wantAccountID, gotAccountID)
		})
	}
}

// TestRateLimiter asserts the fixed-window limiter admits up to the limit then
// returns 429, and that a fresh window resets the count.
func TestRateLimiter(t *testing.T) {
	t.Parallel()

	rl := middlewares.NewRateLimiter(slog.Default(), 2, 100*time.Millisecond)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := rl.Middleware(next)

	call := func() int {
		req := httptest.NewRequest(http.MethodGet, "/apis/gallery/weekly", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	require.Equal(t, http.StatusOK, call())
	require.Equal(t, http.StatusOK, call())
	require.Equal(t, http.StatusTooManyRequests, call())

	time.Sleep(150 * time.Millisecond)
	require.Equal(t, http.StatusOK, call(), "window should reset")
}
