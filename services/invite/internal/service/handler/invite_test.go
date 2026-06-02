package handler_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"
	"application/internal/service/handler"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newMux(t *testing.T, uc *mocks.MockUsecaseInvite) *http.ServeMux {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	h := handler.NewInvite(logger, mux, uc)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

func TestRedeemHandler_HappyPath(t *testing.T) {
	t.Parallel()

	uc := mocks.NewMockUsecaseInvite(t)
	uc.EXPECT().
		Redeem(mock.Anything, "GOODCODE12", "acct-1").
		Return(biz.RedeemResult{Code: "GOODCODE12", IssuerAccountID: "acct-issuer", RedeemedBy: "acct-1"}, nil)

	mux := newMux(t, uc)

	req := httptest.NewRequest(http.MethodPost, "/apis/invites/redeem",
		strings.NewReader(`{"code":"GOODCODE12"}`))
	req.Header.Set("X-Account-Id", "acct-1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "acct-issuer")
}

func TestRedeemHandler_MissingAccount(t *testing.T) {
	t.Parallel()

	uc := mocks.NewMockUsecaseInvite(t)
	mux := newMux(t, uc)

	req := httptest.NewRequest(http.MethodPost, "/apis/invites/redeem",
		strings.NewReader(`{"code":"GOODCODE12"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRedeemHandler_InvalidCode(t *testing.T) {
	t.Parallel()

	uc := mocks.NewMockUsecaseInvite(t)
	uc.EXPECT().
		Redeem(mock.Anything, "USED000000", "acct-1").
		Return(biz.RedeemResult{}, biz.ErrResourceInvalid)

	mux := newMux(t, uc)

	req := httptest.NewRequest(http.MethodPost, "/apis/invites/redeem",
		strings.NewReader(`{"code":"USED000000"}`))
	req.Header.Set("X-Account-Id", "acct-1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestIssueHandler_HappyPath(t *testing.T) {
	t.Parallel()

	uc := mocks.NewMockUsecaseInvite(t)
	uc.EXPECT().
		Issue(mock.Anything, "acct-1").
		Return(entity.Invite{Code: "NEWCODE123", IssuerAccountID: "acct-1", Status: entity.InviteStatusIssued}, nil)

	mux := newMux(t, uc)

	req := httptest.NewRequest(http.MethodPost, "/apis/invites", nil)
	req.Header.Set("X-Account-Id", "acct-1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Contains(t, rr.Body.String(), "NEWCODE123")
}

func TestAdminRevokeHandler(t *testing.T) {
	t.Parallel()

	uc := mocks.NewMockUsecaseInvite(t)
	uc.EXPECT().Revoke(mock.Anything, "CODE1").Return(nil)

	mux := newMux(t, uc)

	req := httptest.NewRequest(http.MethodPost, "/apis/admin/invites/CODE1/revoke", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdminChainHandler(t *testing.T) {
	t.Parallel()

	uc := mocks.NewMockUsecaseInvite(t)
	uc.EXPECT().Chain(mock.Anything, "acct-issuer").Return([]entity.InviteEdge{
		{Code: "C1", InviterAccountID: "acct-issuer", InviteeAccountID: "acct-a"},
	}, nil)

	mux := newMux(t, uc)

	req := httptest.NewRequest(http.MethodGet, "/apis/admin/invites/chain/acct-issuer", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "acct-a")
}
