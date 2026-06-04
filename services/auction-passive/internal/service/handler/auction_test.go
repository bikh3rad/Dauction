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
	"strings"
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
	h := handler.NewAuctionHandler(testLogger(), mux, uc)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

func openAuction() entity.Auction {
	return entity.Auction{
		ID:       uuid.New(),
		LotID:    uuid.New(),
		Atype:    entity.ModeUniqBid,
		State:    entity.StateOpen,
		ClosesAt: time.Now().UTC().Add(24 * time.Hour),
	}
}

func TestGetAuction(t *testing.T) {
	t.Parallel()

	a := openAuction()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Get(mock.Anything, a.ID).Return(a, 3, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/auctions/"+a.ID.String(), nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "UNIQBID", body["atype"])
		require.Equal(t, float64(3), body["participantCount"])
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
		uc.EXPECT().Get(mock.Anything, a.ID).Return(entity.Auction{}, 0, biz.ErrResourceNotFound)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/auctions/"+a.ID.String(), nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestPlaceBid(t *testing.T) {
	t.Parallel()

	a := openAuction()
	bidder := uuid.New()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().
			PlaceBid(mock.Anything, mock.MatchedBy(func(in biz.PlaceBidInput) bool {
				return in.AuctionID == a.ID && in.BidderID == bidder && in.PriceCents == 2500
			})).
			Return(entity.PassiveBid{ID: uuid.New(), AuctionID: a.ID, PriceCents: 2500, PlacedAt: time.Now()}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/auctions/"+a.ID.String()+"/bid",
			strings.NewReader(`{"priceCents":2500,"requestId":"r1"}`))
		req.Header.Set("X-Account-Id", bidder.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("missing caller -> 401", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/auctions/"+a.ID.String()+"/bid",
			strings.NewReader(`{"priceCents":2500}`))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("non-positive price -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/auctions/"+a.ID.String()+"/bid",
			strings.NewReader(`{"priceCents":0}`))
		req.Header.Set("X-Account-Id", bidder.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("out of credits -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().PlaceBid(mock.Anything, mock.Anything).
			Return(entity.PassiveBid{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/auctions/"+a.ID.String()+"/bid",
			strings.NewReader(`{"priceCents":100}`))
		req.Header.Set("X-Account-Id", bidder.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestStanding(t *testing.T) {
	t.Parallel()

	a := openAuction()
	bidder := uuid.New()

	uc := usecasemocks.NewMockUsecaseAuction(t)
	uc.EXPECT().Standing(mock.Anything, a.ID, bidder).Return(biz.Standing{
		Auction: a,
		Prices: []biz.StandingPrice{
			{PriceCents: 1500, IsLowestUnique: true, PlacedAt: time.Now()},
		},
	}, nil)

	mux := newServer(t, uc)
	req := httptest.NewRequest(http.MethodGet, "/apis/auctions/"+a.ID.String()+"/standing", nil)
	req.Header.Set("X-Account-Id", bidder.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	prices, ok := body["prices"].([]any)
	require.True(t, ok)
	require.Len(t, prices, 1)
}

func TestStanding_MissingCaller(t *testing.T) {
	t.Parallel()

	uc := usecasemocks.NewMockUsecaseAuction(t)
	mux := newServer(t, uc)
	req := httptest.NewRequest(http.MethodGet, "/apis/auctions/"+uuid.New().String()+"/standing", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminClose(t *testing.T) {
	t.Parallel()

	a := openAuction()
	winner := uuid.New()

	t.Run("resolved with winner", func(t *testing.T) {
		t.Parallel()

		resolved := a
		resolved.State = entity.StateResolved
		resolved.WinnerAccountID = &winner
		resolved.ClearedPriceCents = 7000

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Close(mock.Anything, a.ID).Return(resolved, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/auctions/"+a.ID.String()+"/close", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "RESOLVED", body["state"])
	})

	t.Run("not open -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseAuction(t)
		uc.EXPECT().Close(mock.Anything, a.ID).Return(entity.Auction{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/auctions/"+a.ID.String()+"/close", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
