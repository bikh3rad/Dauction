package handler_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"
	"application/internal/service/handler"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newServer wires the account handler onto a mux for in-process testing.
func newServer(t *testing.T, uc *mocks.MockUsecaseAccount) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	h := handler.NewAccountHandler(testLogger(), mux, uc)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

func TestAccountHandler_Me(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path returns account", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseAccount(t)
		uc.EXPECT().Get(mock.Anything, id).
			Return(entity.Account{ID: id, Tier: entity.TierMember, KycStatus: entity.KycApproved}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/me", nil)
		req.Header.Set("X-Account-Id", id.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "MEMBER", body["tier"])
		require.Equal(t, true, body["eligible"])
	})

	t.Run("missing X-Account-Id is unauthorized", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseAccount(t)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/me", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAccountHandler_GrantVIP(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseAccount(t)
		uc.EXPECT().GrantVIP(mock.Anything, id).
			Return(entity.Account{ID: id, Tier: entity.TierVIP, KycStatus: entity.KycApproved}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/accounts/"+id.String()+"/vip", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("illegal transition -> 400", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseAccount(t)
		uc.EXPECT().GrantVIP(mock.Anything, id).
			Return(entity.Account{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/accounts/"+id.String()+"/vip", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("bad uuid -> 400", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseAccount(t)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/accounts/not-a-uuid/vip", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAccountHandler_Access(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	uc := mocks.NewMockUsecaseAccount(t)
	uc.EXPECT().Get(mock.Anything, id).
		Return(entity.Account{ID: id, Tier: entity.TierGuest, KycStatus: entity.KycPending}, nil)

	mux := newServer(t, uc)
	req := httptest.NewRequest(http.MethodGet, "/apis/internal/accounts/"+id.String()+"/access", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "GUEST", body["tier"])
	require.Equal(t, false, body["eligible"])
}
