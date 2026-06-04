package handler_test

import (
	"application/internal/biz"
	"application/internal/entity"
	usecasemocks "application/internal/mocks/usecase"
	"application/internal/service/handler"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newServer(t *testing.T, uc *usecasemocks.MockUsecaseAuction) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	h := handler.NewAuctionHandler(testLogger(), mux, uc, biz.NewWallClock())
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

// withCaller sets the gateway-injected identity headers on a request.
func withCaller(req *http.Request, account uuid.UUID, tier string, kyc bool) *http.Request {
	req.Header.Set("X-Account-Id", account.String())
	req.Header.Set("X-Account-Tier", tier)
	if kyc {
		req.Header.Set("X-Kyc-Approved", "true")
	}

	return req
}

func TestAuctionHandler_Get(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path returns server price", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Get(mock.Anything, id).Return(biz.AuctionView{
			Auction:      entity.Auction{ID: id, State: entity.AuctionOpen, CeilingCents: 1000, FloorCents: 100},
			CurrentPrice: 950,
		}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/auctions/"+id.String(), nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "OPEN", body["state"])
		require.Equal(t, float64(950), body["currentPriceCents"])
	})

	t.Run("bad uuid -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/auctions/not-a-uuid", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found -> 404", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Get(mock.Anything, id).Return(biz.AuctionView{}, biz.ErrResourceNotFound)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/auctions/"+id.String(), nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestAuctionHandler_Reserve(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	account := uuid.New()

	t.Run("eligible caller -> 201", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().
			Reserve(mock.Anything, id, mock.MatchedBy(func(in biz.ReserveInput) bool {
				return in.AccountID == account && in.Tier == entity.TierMember && in.KycApproved
			})).
			Return(entity.Reservation{ID: uuid.New(), Kind: entity.KindDeposit10, AmountCents: 100}, nil)

		mux := newServer(t, uc)
		req := withCaller(httptest.NewRequest(http.MethodPost, "/apis/auctions/"+id.String()+"/reserve", nil),
			account, "MEMBER", true)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("missing caller header -> 401", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/auctions/"+id.String()+"/reserve", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("ineligible -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Reserve(mock.Anything, id, mock.Anything).Return(entity.Reservation{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := withCaller(httptest.NewRequest(http.MethodPost, "/apis/auctions/"+id.String()+"/reserve", nil),
			account, "GUEST", false)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAuctionHandler_Lock(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	account := uuid.New()

	uc := usecasemocks.NewMockUsecaseAuction(t)
	uc.EXPECT().Lock(mock.Anything, id, mock.Anything).
		Return(entity.Reservation{Kind: entity.KindFullLock, AmountCents: 1000}, nil)

	mux := newServer(t, uc)
	req := withCaller(httptest.NewRequest(http.MethodPost, "/apis/auctions/"+id.String()+"/lock", nil),
		account, "VIP", true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestAuctionHandler_Buy(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	account := uuid.New()

	t.Run("happy hammer -> 200", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		open := time.Now().UTC()
		uc.EXPECT().Buy(mock.Anything, id, account).Return(entity.Auction{
			ID: id, State: entity.AuctionHammer, OpenAt: &open,
		}, nil)

		mux := newServer(t, uc)
		req := withCaller(httptest.NewRequest(http.MethodPost, "/apis/auctions/"+id.String()+"/buy", nil),
			account, "MEMBER", true)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "HAMMER", body["state"])
	})

	t.Run("missing caller -> 401", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/auctions/"+id.String()+"/buy", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("rejected (not open / already hammered) -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Buy(mock.Anything, id, account).Return(entity.Auction{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := withCaller(httptest.NewRequest(http.MethodPost, "/apis/auctions/"+id.String()+"/buy", nil),
			account, "MEMBER", true)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestAuctionHandler_AdminLifecycle(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("open -> 200", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Open(mock.Anything, id).Return(entity.Auction{ID: id, State: entity.AuctionOpen}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/auctions/"+id.String()+"/open", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("complete -> 200", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Complete(mock.Anything, id).Return(entity.Auction{ID: id, State: entity.AuctionCompleted}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/auctions/"+id.String()+"/complete", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("abort illegal -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Abort(mock.Anything, id).Return(entity.Auction{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/auctions/"+id.String()+"/abort", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
