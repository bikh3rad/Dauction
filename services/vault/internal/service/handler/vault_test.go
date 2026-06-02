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
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newServer wires the vault handler onto a mux for in-process testing.
func newServer(t *testing.T, uc *mocks.MockUsecaseVault) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	h := handler.NewVaultHandler(testLogger(), mux, uc)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

func TestVaultHandler_View(t *testing.T) {
	t.Parallel()

	owner := uuid.New()

	t.Run("happy path returns objects + balance", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseVault(t)
		uc.EXPECT().View(mock.Anything, owner).Return(biz.VaultView{
			Objects:            []entity.VaultObject{{ID: uuid.New(), OwnerAccountID: owner, State: entity.ObjectInVault, AppraisedValueCents: 1000}},
			CreditBalanceCents: 8500,
		}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/vault", nil)
		req.Header.Set("X-Account-Id", owner.String())
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, float64(8500), body["creditBalanceCents"])
		require.Len(t, body["objects"], 1)
	})

	t.Run("missing header is access denied", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseVault(t)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/vault", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestVaultHandler_Create(t *testing.T) {
	t.Parallel()

	owner := uuid.New()

	uc := mocks.NewMockUsecaseVault(t)
	uc.EXPECT().AddObject(mock.Anything, owner, "Rolex", "desc", int64(620000)).
		Return(entity.VaultObject{ID: uuid.New(), OwnerAccountID: owner, State: entity.ObjectInVault, AppraisedValueCents: 620000}, nil)

	mux := newServer(t, uc)
	body := `{"title":"Rolex","description":"desc","appraisedValueCents":620000}`
	req := httptest.NewRequest(http.MethodPost, "/apis/vault/objects", strings.NewReader(body))
	req.Header.Set("X-Account-Id", owner.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestVaultHandler_List(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	objectID := uuid.New()

	t.Run("vickrey with duration", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseVault(t)
		uc.EXPECT().
			List(mock.Anything, owner, objectID, biz.ListRequest{Mode: entity.AuctionVickrey, DurationDays: 5}).
			Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectAppraising}, nil)

		mux := newServer(t, uc)
		body := `{"atype":"VICKREY","durationDays":5}`
		req := httptest.NewRequest(http.MethodPost, "/apis/vault/objects/"+objectID.String()+"/list", strings.NewReader(body))
		req.Header.Set("X-Account-Id", owner.String())
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("biz invalid surfaces 400", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseVault(t)
		uc.EXPECT().
			List(mock.Anything, owner, objectID, biz.ListRequest{Mode: entity.AuctionDutch, DurationDays: 5}).
			Return(entity.VaultObject{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		body := `{"atype":"DUTCH","durationDays":5}`
		req := httptest.NewRequest(http.MethodPost, "/apis/vault/objects/"+objectID.String()+"/list", strings.NewReader(body))
		req.Header.Set("X-Account-Id", owner.String())
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not owner surfaces 401", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseVault(t)
		uc.EXPECT().
			List(mock.Anything, owner, objectID, biz.ListRequest{Mode: entity.AuctionDutch}).
			Return(entity.VaultObject{}, biz.ErrResourceAccessDenied)

		mux := newServer(t, uc)
		body := `{"atype":"DUTCH"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/vault/objects/"+objectID.String()+"/list", strings.NewReader(body))
		req.Header.Set("X-Account-Id", owner.String())
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestVaultHandler_Buyback(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	objectID := uuid.New()

	uc := mocks.NewMockUsecaseVault(t)
	uc.EXPECT().
		Buyback(mock.Anything, owner, objectID, entity.BuybackModeCredit).
		Return(biz.BuybackResult{
			Object:       entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectBoughtBack},
			Mode:         entity.BuybackModeCredit,
			PayoutCents:  850,
			BalanceCents: 850,
		}, nil)

	mux := newServer(t, uc)
	body := `{"mode":"CREDIT"}`
	req := httptest.NewRequest(http.MethodPost, "/apis/vault/objects/"+objectID.String()+"/buyback", strings.NewReader(body))
	req.Header.Set("X-Account-Id", owner.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, float64(850), resp["payoutCents"])
	require.Equal(t, "CREDIT", resp["mode"])
}

func TestVaultHandler_BadObjectID(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	uc := mocks.NewMockUsecaseVault(t)

	mux := newServer(t, uc)
	req := httptest.NewRequest(http.MethodPost, "/apis/vault/objects/not-a-uuid/buyback", strings.NewReader(`{"mode":"CASH"}`))
	req.Header.Set("X-Account-Id", owner.String())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
