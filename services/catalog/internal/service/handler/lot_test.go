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

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newServer(t *testing.T, uc *usecasemocks.MockUsecaseLot) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	h := handler.NewLotHandler(testLogger(), mux, uc)
	require.NoError(t, h.RegisterHandler(context.Background()))

	return mux
}

func TestLotHandler_Weekly(t *testing.T) {
	t.Parallel()

	uc := usecasemocks.NewMockUsecaseLot(t)
	uc.EXPECT().GetWeekly(mock.Anything, "2026-W23").
		Return([]entity.Lot{{ID: uuid.New(), Mode: entity.ModeDutch, State: entity.LotScheduled, ISOWeek: "2026-W23"}}, nil)

	mux := newServer(t, uc)
	req := httptest.NewRequest(http.MethodGet, "/apis/gallery/weekly?week=2026-W23", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "2026-W23", body["week"])
	require.Equal(t, float64(32), body["supplyCap"])
	require.Len(t, body["lots"], 1)
}

func TestLotHandler_GetLot(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path with attestation summary", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseLot(t)
		uc.EXPECT().Get(mock.Anything, id).Return(
			entity.Lot{ID: id, Mode: entity.ModeVickrey, State: entity.LotCertified},
			[]entity.Attestation{{ID: uuid.New(), LotID: id, Result: entity.AttestPass}},
			nil,
		)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/lots/"+id.String(), nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, true, body["certified"])
	})

	t.Run("not found -> 404", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseLot(t)
		uc.EXPECT().Get(mock.Anything, id).Return(entity.Lot{}, nil, biz.ErrResourceNotFound)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/lots/"+id.String(), nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("bad uuid -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseLot(t)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodGet, "/apis/lots/not-a-uuid", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLotHandler_Attest(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	inspector := uuid.New()

	t.Run("happy path -> 201", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseLot(t)
		uc.EXPECT().Attest(mock.Anything, id, mock.MatchedBy(func(in biz.AttestInput) bool {
			return in.InspectorID == inspector && in.Result == entity.AttestPass
		})).Return(entity.Attestation{ID: uuid.New(), LotID: id, Result: entity.AttestPass}, nil)

		mux := newServer(t, uc)
		body := `{"inspectorId":"` + inspector.String() + `","result":"PASS","notesRef":"ref"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/lots/"+id.String()+"/attest", strings.NewReader(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("invalid result -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseLot(t)

		mux := newServer(t, uc)
		body := `{"inspectorId":"` + inspector.String() + `","result":"MAYBE"}`
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/lots/"+id.String()+"/attest", strings.NewReader(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLotHandler_Certify(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseLot(t)
		uc.EXPECT().Certify(mock.Anything, id).
			Return(entity.Lot{ID: id, State: entity.LotCertified}, nil)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/lots/"+id.String()+"/certify", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("certification gate -> 400", func(t *testing.T) {
		t.Parallel()

		uc := usecasemocks.NewMockUsecaseLot(t)
		uc.EXPECT().Certify(mock.Anything, id).
			Return(entity.Lot{}, biz.ErrResourceInvalid)

		mux := newServer(t, uc)
		req := httptest.NewRequest(http.MethodPost, "/apis/admin/lots/"+id.String()+"/certify", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLotHandler_Schedule_CapReached(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	uc := usecasemocks.NewMockUsecaseLot(t)
	uc.EXPECT().Schedule(mock.Anything, id, mock.Anything).
		Return(entity.Lot{}, biz.ErrResourceInvalid)

	mux := newServer(t, uc)
	req := httptest.NewRequest(http.MethodPost, "/apis/admin/lots/"+id.String()+"/schedule", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
