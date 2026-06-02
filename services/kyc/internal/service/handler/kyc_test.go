package handler_test

import (
	"application/internal/biz"
	"application/internal/entity"
	mocks "application/internal/mocks/usecase"
	"application/internal/service/handler"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var account = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func newHandler(t *testing.T) (*http.ServeMux, *mocks.MockUsecaseKyc) {
	t.Helper()

	uc := mocks.NewMockUsecaseKyc(t)
	mux := http.NewServeMux()
	h := handler.NewKyc(slog.Default(), mux, uc, biz.OutboxRelayMarker{})
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux, uc
}

func TestStartHandler(t *testing.T) {
	t.Parallel()

	mux, uc := newHandler(t)

	uc.EXPECT().Start(mock.Anything, mock.MatchedBy(func(p biz.StartParams) bool {
		return p.AccountID == account && p.DocType == entity.DocEmiratesID
	})).Return(biz.StartResult{SubmissionID: uuid.New(), ChallengeID: uuid.New()}, nil)

	body := `{"docType":"EMIRATES_ID","docRef":"ref-1","phone":"+971500000000"}`
	req := httptest.NewRequest(http.MethodPost, "/apis/kyc/start", strings.NewReader(body))
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestStartHandlerMissingAccount(t *testing.T) {
	t.Parallel()

	mux, _ := newHandler(t)

	body := `{"docType":"EMIRATES_ID","docRef":"ref-1","phone":"+971500000000"}`
	req := httptest.NewRequest(http.MethodPost, "/apis/kyc/start", strings.NewReader(body))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStartHandlerInvalidDocType(t *testing.T) {
	t.Parallel()

	mux, _ := newHandler(t)

	body := `{"docType":"NOPE","docRef":"ref-1","phone":"+971500000000"}`
	req := httptest.NewRequest(http.MethodPost, "/apis/kyc/start", strings.NewReader(body))
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStartHandlerConflict(t *testing.T) {
	t.Parallel()

	mux, uc := newHandler(t)

	uc.EXPECT().Start(mock.Anything, mock.Anything).
		Return(biz.StartResult{}, biz.ErrResourceExists)

	body := `{"docType":"PASSPORT","docRef":"ref-1","phone":"+971500000000"}`
	req := httptest.NewRequest(http.MethodPost, "/apis/kyc/start", strings.NewReader(body))
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestVerifyHandler(t *testing.T) {
	t.Parallel()

	mux, uc := newHandler(t)

	uc.EXPECT().Verify(mock.Anything, account, "123456").
		Return(entity.Submission{ID: uuid.New(), AccountID: account, State: entity.SubmissionSubmitted}, nil)

	req := httptest.NewRequest(http.MethodPost, "/apis/kyc/verify", strings.NewReader(`{"code":"123456"}`))
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestVerifyHandlerBadCode(t *testing.T) {
	t.Parallel()

	mux, _ := newHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/apis/kyc/verify", strings.NewReader(`{"code":"12"}`))
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStatusHandlerNotFound(t *testing.T) {
	t.Parallel()

	mux, uc := newHandler(t)

	uc.EXPECT().Status(mock.Anything, account).
		Return(entity.Submission{}, biz.ErrResourceNotFound)

	req := httptest.NewRequest(http.MethodGet, "/apis/kyc/status", nil)
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminQueueHandler(t *testing.T) {
	t.Parallel()

	mux, uc := newHandler(t)

	uc.EXPECT().PendingQueue(mock.Anything).
		Return([]entity.Submission{{ID: uuid.New(), AccountID: account, State: entity.SubmissionSubmitted}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/apis/admin/kyc", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminApproveHandler(t *testing.T) {
	t.Parallel()

	mux, uc := newHandler(t)

	id := uuid.New()
	uc.EXPECT().Approve(mock.Anything, id, account).
		Return(entity.Submission{ID: id, State: entity.SubmissionApproved}, nil)

	req := httptest.NewRequest(http.MethodPost, "/apis/admin/kyc/"+id.String()+"/approve", nil)
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminApproveHandlerBadID(t *testing.T) {
	t.Parallel()

	mux, _ := newHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/apis/admin/kyc/not-a-uuid/approve", nil)
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminRejectHandler(t *testing.T) {
	t.Parallel()

	mux, uc := newHandler(t)

	id := uuid.New()
	uc.EXPECT().Reject(mock.Anything, mock.MatchedBy(func(p biz.RejectParams) bool {
		return p.SubmissionID == id && p.Reason == "KYC_REJECTED"
	})).Return(entity.Submission{ID: id, State: entity.SubmissionRejected, RejectionReason: "KYC_REJECTED"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/apis/admin/kyc/"+id.String()+"/reject", strings.NewReader(`{"reason":"KYC_REJECTED"}`))
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminRejectHandlerMissingReason(t *testing.T) {
	t.Parallel()

	mux, _ := newHandler(t)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/apis/admin/kyc/"+id.String()+"/reject", strings.NewReader(`{}`))
	req.Header.Set("X-Account-Id", account.String())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
