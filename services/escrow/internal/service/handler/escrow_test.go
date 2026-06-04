package handler_test

import (
	"application/internal/biz"
	"application/internal/entity"
	usecasemocks "application/internal/mocks/usecase"
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

// newServer wires the escrow handler onto a mux for in-process testing.
func newServer(t *testing.T, uc *usecasemocks.MockUsecaseEscrow) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	h := handler.NewEscrowHandler(testLogger(), mux, uc)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

func body(t *testing.T, v any) *bytes.Reader {
	t.Helper()

	raw, err := json.Marshal(v)
	require.NoError(t, err)

	return bytes.NewReader(raw)
}

func TestEscrowHandler_Get(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path returns trade", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		uc.EXPECT().Get(mock.Anything, id).Return(biz.TradeView{
			Trade:        entity.EscrowTrade{ID: id, Kind: entity.KindPassive, State: entity.StateHeld, PriceCents: 100, PremiumCents: 5},
			Balances:     []entity.ParticipantBalance{{ParticipantAccountID: uuid.New(), BalanceCents: 105}},
			Conservation: entity.Conservation{Inflows: 105, Disbursed: 0},
		}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/escrow/"+id.String(), nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "HELD", resp["state"])
		require.Equal(t, float64(105), resp["obligationCents"])
	})

	t.Run("bad trade id -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/escrow/not-a-uuid", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unknown trade -> 404", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		uc.EXPECT().Get(mock.Anything, id).Return(biz.TradeView{}, biz.ErrResourceNotFound)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/escrow/"+id.String(), nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestEscrowHandler_Fund(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	caller := uuid.New()

	t.Run("happy path -> HELD", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		uc.EXPECT().Fund(mock.Anything, id, caller, int64(105)).
			Return(entity.EscrowTrade{ID: id, State: entity.StateHeld}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+id.String()+"/fund", body(t, map[string]int64{"amountCents": 105}))
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "HELD", resp["state"])
	})

	t.Run("missing X-Account-Id -> 401", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+id.String()+"/fund", body(t, map[string]int64{"amountCents": 105}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("wrong amount -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		uc.EXPECT().Fund(mock.Anything, id, caller, int64(1)).
			Return(entity.EscrowTrade{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+id.String()+"/fund", body(t, map[string]int64{"amountCents": 1}))
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestEscrowHandler_Confirm(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	caller := uuid.New()

	t.Run("happy path -> RELEASED", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		uc.EXPECT().Confirm(mock.Anything, id, caller, entity.ReleaseVaultCredit).
			Return(entity.EscrowTrade{ID: id, State: entity.StateReleased, ReleaseMode: entity.ReleaseVaultCredit}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+id.String()+"/confirm", body(t, map[string]string{"mode": "VAULT_CREDIT"}))
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "RELEASED", resp["state"])
		require.Equal(t, "VAULT_CREDIT", resp["releaseMode"])
	})

	t.Run("disputed blocks confirm -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		uc.EXPECT().Confirm(mock.Anything, id, caller, entity.ReleaseCash).
			Return(entity.EscrowTrade{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+id.String()+"/confirm", body(t, map[string]string{"mode": "CASH"}))
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestEscrowHandler_AdminRefundForfeit(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	participant := uuid.New()

	t.Run("refund happy path", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		uc.EXPECT().Refund(mock.Anything, id, participant).
			Return(entity.EscrowTrade{ID: id, State: entity.StateRefunded}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/escrow/"+id.String()+"/refund", body(t, map[string]string{"participantId": participant.String()}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "REFUNDED", resp["state"])
	})

	t.Run("forfeit happy path", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseEscrow(t)
		uc.EXPECT().Forfeit(mock.Anything, id).
			Return(entity.EscrowTrade{ID: id, State: entity.StateForfeited}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/escrow/"+id.String()+"/forfeit", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "FORFEITED", resp["state"])
	})
}
