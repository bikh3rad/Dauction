package handler_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"application/app"
	"application/internal/biz"
	mocks "application/internal/mocks/usecase"
	"application/internal/service/handler"
	"application/pkg/middlewares"

	"log/slog"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newGateway wires a ProxyHandler against a single fake upstream (pointed at by
// every service name) and the supplied access usecase, with rate-limiting off.
func newGateway(t *testing.T, accessUC biz.UsecaseAccess, upstreamURL string) http.Handler {
	t.Helper()

	mux := http.NewServeMux()
	upstreams := &app.UpstreamsConfig{
		Identity:       upstreamURL,
		Kyc:            upstreamURL,
		Vault:          upstreamURL,
		Catalog:        upstreamURL,
		AuctionDutch:   upstreamURL,
		AuctionPassive: upstreamURL,
		Bids:           upstreamURL,
		Escrow:         upstreamURL,
		Dispute:        upstreamURL,
		Notifier:       upstreamURL,
	}
	rl := &app.RateLimitConfig{Enabled: false}

	h := handler.NewProxyHandler(slog.Default(), mux, upstreams, rl, accessUC)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

// TestProxyPublicRoute: a public gallery read is proxied without any guard call.
func TestProxyPublicRoute(t *testing.T) {
	t.Parallel()

	var gotPath, gotAccount string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAccount = r.Header.Get(middlewares.HeaderAccountID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("gallery"))
	}))
	defer upstream.Close()

	accessUC := mocks.NewMockUsecaseAccess(t)
	accessUC.EXPECT().
		Authorize(mock.Anything, "", mock.Anything).
		Return(biz.GuardResult{Allowed: true}, nil).
		Once()

	gw := newGateway(t, accessUC, upstream.URL)

	req := httptest.NewRequest(http.MethodGet, "/apis/gallery/weekly", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "/apis/gallery/weekly", gotPath)
	require.Empty(t, gotAccount, "public route must not inject identity header")
	body, _ := io.ReadAll(rec.Body)
	require.Equal(t, "gallery", string(body))
}

// TestProxyInjectsTrustedHeaders: a participation route, once authorized, forwards
// the upstream call with trusted X-Account-Tier / X-Kyc-Approved headers.
func TestProxyInjectsTrustedHeaders(t *testing.T) {
	t.Parallel()

	var tier, kyc, account string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tier = r.Header.Get(middlewares.HeaderAccountTier)
		kyc = r.Header.Get(middlewares.HeaderKycApproved)
		account = r.Header.Get(middlewares.HeaderAccountID)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	accessUC := mocks.NewMockUsecaseAccess(t)
	accessUC.EXPECT().
		Authorize(mock.Anything, "acc-9", mock.Anything).
		Return(biz.GuardResult{
			Allowed: true,
			Access:  accessVIPApproved(),
		}, nil).
		Once()

	gw := newGateway(t, accessUC, upstream.URL)

	req := httptest.NewRequest(http.MethodPost, "/apis/auctions/A1/bid", nil)
	req.Header.Set("Authorization", "Bearer acc-9")
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "acc-9", account)
	require.Equal(t, "VIP", tier)
	require.Equal(t, "true", kyc)
}

// TestProxyGuardDenied: a denied guard short-circuits with the language-neutral
// code and never reaches the upstream.
func TestProxyGuardDenied(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("upstream must not be called when guard denies")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	accessUC := mocks.NewMockUsecaseAccess(t)
	accessUC.EXPECT().
		Authorize(mock.Anything, "acc-guest", mock.Anything).
		Return(biz.GuardResult{}, biz.ErrTierRequired).
		Once()

	gw := newGateway(t, accessUC, upstream.URL)

	req := httptest.NewRequest(http.MethodPost, "/apis/auctions/A1/bid", nil)
	req.Header.Set("Authorization", "Bearer acc-guest")
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "TIER_REQUIRED")
}

// TestProxyNoRoute: an unmapped /apis path returns 404 NOT_FOUND.
func TestProxyNoRoute(t *testing.T) {
	t.Parallel()

	accessUC := mocks.NewMockUsecaseAccess(t)
	gw := newGateway(t, accessUC, "http://unused")

	req := httptest.NewRequest(http.MethodGet, "/apis/does-not-exist", nil)
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "NOT_FOUND")
}
