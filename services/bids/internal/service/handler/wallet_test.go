package handler_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"
	"application/internal/service/handler"
	"bytes"
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

// newServer wires the wallet handler onto a mux for in-process testing.
func newServer(t *testing.T, uc *mocks.MockUsecaseWallet) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	h := handler.NewWalletHandler(testLogger(), mux, uc)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

func TestWalletHandler_Wallet(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path returns balance", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)
		uc.EXPECT().Wallet(mock.Anything, id, 0).
			Return(biz.WalletView{Wallet: entity.BidWallet{AccountID: id, BalanceCredits: 42}}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/bids/wallet", nil)
		req.Header.Set("X-Account-Id", id.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.EqualValues(t, 42, body["balanceCredits"])
	})

	t.Run("missing X-Account-Id is unauthorized", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/bids/wallet", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestWalletHandler_Packages(t *testing.T) {
	t.Parallel()

	uc := mocks.NewMockUsecaseWallet(t)
	uc.EXPECT().Packages(mock.Anything).Return([]entity.BidPackage{
		{ID: "PKG_100", Credits: 100, PriceCents: 8000, BestValue: true},
	}, nil)

	mux := newServer(t, uc)
	req := httptest.NewRequest(http.MethodGet, "/apis/bids/packages", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 1)
	require.Equal(t, "PKG_100", body[0]["id"])
	require.EqualValues(t, 8000, body[0]["priceCents"])
}

func TestWalletHandler_Buy(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)
		uc.EXPECT().Buy(mock.Anything, id, "PKG_50", "").
			Return(biz.PurchaseResult{CreditsGranted: 50, USDCChargedCents: 4500, Balance: 50}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/bids/buy",
			bytes.NewBufferString(`{"packageId":"PKG_50"}`))
		req.Header.Set("X-Account-Id", id.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.EqualValues(t, 50, body["creditsGranted"])
		require.EqualValues(t, 4500, body["usdcChargedCents"])
	})

	t.Run("missing packageId is invalid", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/bids/buy", bytes.NewBufferString(`{}`))
		req.Header.Set("X-Account-Id", id.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unknown package surfaces invalid", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)
		uc.EXPECT().Buy(mock.Anything, id, "PKG_NOPE", "").
			Return(biz.PurchaseResult{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/bids/buy",
			bytes.NewBufferString(`{"packageId":"PKG_NOPE"}`))
		req.Header.Set("X-Account-Id", id.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing X-Account-Id is unauthorized", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/bids/buy",
			bytes.NewBufferString(`{"packageId":"PKG_50"}`))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestWalletHandler_Debit(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path debits and returns balance", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)
		uc.EXPECT().Debit(mock.Anything, id, int64(1), "bid-1", "auction-9").
			Return(biz.DebitResult{Amount: 1, Balance: 41}, nil)

		mux := newServer(t, uc)
		body := `{"accountId":"` + id.String() + `","amount":1,"idempotencyKey":"bid-1","auctionId":"auction-9"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/internal/bids/debit", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.EqualValues(t, 41, resp["balanceCredits"])
	})

	t.Run("out of credits is bad request", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)
		uc.EXPECT().Debit(mock.Anything, id, int64(1), "bid-2", "auction-9").
			Return(biz.DebitResult{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		body := `{"accountId":"` + id.String() + `","amount":1,"idempotencyKey":"bid-2","auctionId":"auction-9"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/internal/bids/debit", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing idempotency key is bad request", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)

		mux := newServer(t, uc)
		body := `{"accountId":"` + id.String() + `","amount":1,"auctionId":"auction-9"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/internal/bids/debit", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("bad account id is bad request", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseWallet(t)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/internal/bids/debit",
			bytes.NewBufferString(`{"accountId":"not-a-uuid","amount":1,"idempotencyKey":"bid-3","auctionId":"a"}`))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
