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

func newServer(t *testing.T, uc *mocks.MockUsecaseDispute) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	h := handler.NewDisputeHandler(testLogger(), mux, uc)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

const tradeID = "trade-xyz"

func TestDisputeHandler_Open(t *testing.T) {
	t.Parallel()

	claimant := uuid.New()
	respondent := uuid.New()

	t.Run("happy path returns 201", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().
			Open(mock.Anything, mock.MatchedBy(func(p biz.OpenParams) bool {
				return p.TradeID == tradeID && p.Claimant == claimant && p.Respondent == respondent &&
					p.ReasonCode == entity.ReasonAuthenticity
			})).
			Return(entity.Dispute{ID: uuid.New(), TradeID: tradeID, State: entity.StateOpen}, nil)

		mux := newServer(t, uc)
		body := `{"reasonCode":"AUTHENTICITY","evidenceRef":"ref","respondent":"` + respondent.String() + `"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute", strings.NewReader(body))
		req.Header.Set("X-Account-Id", claimant.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("missing X-Account-Id is unauthorized", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		mux := newServer(t, uc)
		body := `{"reasonCode":"AUTHENTICITY","respondent":"` + respondent.String() + `"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute", strings.NewReader(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("duplicate open returns 409", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().Open(mock.Anything, mock.Anything).Return(entity.Dispute{}, biz.ErrResourceExists)

		mux := newServer(t, uc)
		body := `{"reasonCode":"CONDITION","respondent":"` + respondent.String() + `"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute", strings.NewReader(body))
		req.Header.Set("X-Account-Id", claimant.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("bad respondent uuid returns 400", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		mux := newServer(t, uc)
		body := `{"reasonCode":"OTHER","respondent":"not-a-uuid"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute", strings.NewReader(body))
		req.Header.Set("X-Account-Id", claimant.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestDisputeHandler_Get(t *testing.T) {
	t.Parallel()

	caller := uuid.New()

	t.Run("party reads dispute + trail", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().Get(mock.Anything, tradeID, caller, false).
			Return(biz.DisputeView{
				Dispute: entity.Dispute{ID: uuid.New(), TradeID: tradeID, State: entity.StateOpen},
				Events:  []entity.DisputeEvent{{ID: uuid.New(), Action: entity.ActionOpened}},
			}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/escrow/"+tradeID+"/dispute", nil)
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Contains(t, body, "events")
	})

	t.Run("admin header sets admin flag", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().Get(mock.Anything, tradeID, caller, true).
			Return(biz.DisputeView{Dispute: entity.Dispute{ID: uuid.New(), TradeID: tradeID}}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/escrow/"+tradeID+"/dispute", nil)
		req.Header.Set("X-Account-Id", caller.String())
		req.Header.Set("X-Admin", "true")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().Get(mock.Anything, tradeID, caller, false).
			Return(biz.DisputeView{}, biz.ErrResourceNotFound)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/escrow/"+tradeID+"/dispute", nil)
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestDisputeHandler_AddEvidence(t *testing.T) {
	t.Parallel()

	caller := uuid.New()

	t.Run("happy path returns 204", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().AddEvidence(mock.Anything, tradeID, caller, "ref-1").Return(nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute/evidence",
			strings.NewReader(`{"detailRef":"ref-1"}`))
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("empty detailRef returns 400", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute/evidence",
			strings.NewReader(`{"detailRef":""}`))
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestDisputeHandler_Resolve(t *testing.T) {
	t.Parallel()

	ruledBy := uuid.New()

	t.Run("happy path returns 200", func(t *testing.T) {
		t.Parallel()

		ruling := entity.RulingSplit
		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().Resolve(mock.Anything, tradeID, entity.RulingSplit, ruledBy).
			Return(entity.Dispute{ID: uuid.New(), State: entity.StateResolved, Ruling: &ruling}, nil)

		mux := newServer(t, uc)
		body := `{"ruling":"SPLIT","ruledBy":"` + ruledBy.String() + `"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute/resolve", strings.NewReader(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("not UNDER_REVIEW returns 400", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().Resolve(mock.Anything, tradeID, entity.RulingRefundBuyer, ruledBy).
			Return(entity.Dispute{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		body := `{"ruling":"REFUND_BUYER","ruledBy":"` + ruledBy.String() + `"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute/resolve", strings.NewReader(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("bad ruledBy uuid returns 400", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		mux := newServer(t, uc)
		body := `{"ruling":"SPLIT","ruledBy":"nope"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute/resolve", strings.NewReader(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestDisputeHandler_Withdraw(t *testing.T) {
	t.Parallel()

	caller := uuid.New()

	t.Run("happy path returns 200", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().Withdraw(mock.Anything, tradeID, caller).
			Return(entity.Dispute{ID: uuid.New(), State: entity.StateWithdrawn}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute/withdraw", nil)
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("non-claimant denied", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().Withdraw(mock.Anything, tradeID, caller).
			Return(entity.Dispute{}, biz.ErrResourceAccessDenied)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/escrow/"+tradeID+"/dispute/withdraw", nil)
		req.Header.Set("X-Account-Id", caller.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestDisputeHandler_AdminListAndReview(t *testing.T) {
	t.Parallel()

	t.Run("list filters by state", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().ListByState(mock.Anything, biz.ListFilter{State: entity.StateOpen}).
			Return([]entity.Dispute{{ID: uuid.New(), State: entity.StateOpen}}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/admin/disputes?state=OPEN", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.EqualValues(t, 1, body["count"])
	})

	t.Run("review moves OPEN to UNDER_REVIEW", func(t *testing.T) {
		t.Parallel()

		id := uuid.New()
		admin := uuid.New()
		uc := mocks.NewMockUsecaseDispute(t)
		uc.EXPECT().StartReview(mock.Anything, id, admin).
			Return(entity.Dispute{ID: id, State: entity.StateUnderReview}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/disputes/"+id.String()+"/review", nil)
		req.Header.Set("X-Account-Id", admin.String())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("review with bad id returns 400", func(t *testing.T) {
		t.Parallel()

		uc := mocks.NewMockUsecaseDispute(t)
		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/disputes/not-a-uuid/review", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
