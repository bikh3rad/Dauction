package handler

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/service"
	"application/internal/service/dto"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type lotHandler struct {
	logger *slog.Logger
	mux    *http.ServeMux
	lot    biz.UsecaseLot
}

var _ service.Handler = (*lotHandler)(nil)

// NewLotHandler constructs the catalog HTTP handler.
func NewLotHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseLot) *lotHandler {
	return &lotHandler{
		logger: logger.With("layer", "LotHandler"),
		mux:    mux,
		lot:    uc,
	}
}

// RegisterHandler self-registers the catalog routes (Go 1.22 method patterns).
func (h *lotHandler) RegisterHandler(_ context.Context) error {
	// public gallery reads (no auth)
	h.mux.HandleFunc("GET /apis/gallery/weekly", h.weekly)
	h.mux.HandleFunc("GET /apis/lots/{id}", h.getLot)
	// admin surface
	h.mux.HandleFunc("GET /apis/admin/lots", h.adminList)
	h.mux.HandleFunc("POST /apis/admin/lots/{id}/attest", h.attest)
	h.mux.HandleFunc("POST /apis/admin/lots/{id}/certify", h.certify)
	h.mux.HandleFunc("POST /apis/admin/lots/{id}/schedule", h.schedule)
	// categories (public)
	h.mux.HandleFunc("GET /apis/categories", h.categories)
	// inspector workflow (INSPECTOR role; the auction-eligibility gate, §3.5)
	h.mux.HandleFunc("GET /apis/inspector/queue", h.inspectorQueue)
	h.mux.HandleFunc("POST /apis/inspector/lots/{id}/inspect", h.inspect)

	return nil
}

// weekly returns this ISO week's SCHEDULED lots (public gallery).
//
//	@Summary		Weekly gallery
//	@Description	This ISO week's SCHEDULED lots (public, no auth). Pass ?week=YYYY-Www to read a specific week; defaults to the current ISO week. Returns the 32-lot supply cap so the client can render "n of 32".
//	@Tags			catalog
//	@Produce		json
//	@Param			week	query		string	false	"ISO week, e.g. 2026-W23 (defaults to current)"
//	@Success		200		{object}	dto.WeeklyResp
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/gallery/weekly [get]
func (h *lotHandler) weekly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Weekly")

	week := r.URL.Query().Get("week")

	lots, err := h.lot.GetWeekly(ctx, week)
	if err != nil {
		logger.ErrorContext(ctx, "get weekly failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	resolved := week
	if resolved == "" {
		resolved = biz.ISOWeekOf(time.Now().UTC())
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.WeeklyResp{
		Week:      resolved,
		SupplyCap: biz.WeeklySupplyCap,
		Lots:      dto.ToLotResps(lots),
	})
}

// getLot returns a single lot's detail plus its attestation summary (public).
//
//	@Summary		Lot detail
//	@Description	Public lot detail: atype, params, state, and the attestation summary (the certification gate's evidence).
//	@Tags			catalog
//	@Produce		json
//	@Param			id	path		string	true	"Lot UUID"
//	@Success		200	{object}	dto.LotDetailResp
//	@Failure		400	{object}	dto.ErrorResponse	"Bad UUID"
//	@Failure		404	{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/lots/{id} [get]
func (h *lotHandler) getLot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "GetLot")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	l, atts, err := h.lot.Get(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "get lot failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToLotDetailResp(l, atts))
}

// adminList lists lots, optionally filtered by state and/or ISO week.
//
//	@Summary		Admin lot list
//	@Description	House/admin lot listing, optionally filtered by ?state=DRAFT|CERTIFIED|SCHEDULED|REJECTED and ?week=YYYY-Www.
//	@Tags			catalog-admin
//	@Produce		json
//	@Param			state	query		string	false	"State filter"
//	@Param			week	query		string	false	"ISO week filter"
//	@Success		200		{array}		dto.LotResp
//	@Failure		400		{object}	dto.ErrorResponse	"Bad filter"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/lots [get]
func (h *lotHandler) adminList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "AdminList")

	filter := biz.LotListFilter{
		State:   entity.LotState(r.URL.Query().Get("state")),
		ISOWeek: r.URL.Query().Get("week"),
	}

	lots, err := h.lot.List(ctx, filter)
	if err != nil {
		logger.WarnContext(ctx, "list lots failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToLotResps(lots))
}

// attest records an inspector attestation (PASS/FAIL) on a lot.
//
//	@Summary		Attest a lot
//	@Description	House/inspector records an attestation. A PASS seal unlocks certification (the gate). A FAIL on a DRAFT lot rejects it. Emits attestation.recorded.
//	@Tags			catalog-admin
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Lot UUID"
//	@Param			body	body		dto.AttestRequest	true	"Attestation"
//	@Success		201		{object}	dto.AttestationResp
//	@Failure		400		{object}	dto.ErrorResponse	"Bad request / invalid state"
//	@Failure		404		{object}	dto.ErrorResponse	"Lot not found"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/lots/{id}/attest [post]
func (h *lotHandler) attest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Attest")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	var req dto.AttestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	inspectorID, result, err := req.Validate()
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	att, err := h.lot.Attest(ctx, id, biz.AttestInput{
		InspectorID: inspectorID,
		Result:      result,
		NotesRef:    req.NotesRef,
	})
	if err != nil {
		logger.WarnContext(ctx, "attest failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusCreated, dto.ToAttestationResp(att))
}

// certify moves a DRAFT lot to CERTIFIED, requiring a PASS attestation.
//
//	@Summary		Certify a lot
//	@Description	House certifies a DRAFT lot. The certification gate requires an existing PASS attestation; without one this returns RESOURCE_INVALID. Emits lot.certified.
//	@Tags			catalog-admin
//	@Produce		json
//	@Param			id	path		string	true	"Lot UUID"
//	@Success		200	{object}	dto.LotResp
//	@Failure		400	{object}	dto.ErrorResponse	"No PASS attestation / illegal transition"
//	@Failure		404	{object}	dto.ErrorResponse	"Lot not found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/lots/{id}/certify [post]
func (h *lotHandler) certify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Certify")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	l, err := h.lot.Certify(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "certify failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToLotResp(l))
}

// schedule moves a CERTIFIED lot to SCHEDULED, enforcing the weekly 32-cap.
//
//	@Summary		Schedule a lot
//	@Description	House schedules a CERTIFIED lot into its ISO week's gallery. Enforces the weekly 32-lot supply cap; exceeding it returns RESOURCE_INVALID. Emits lot.scheduled with atype + params.
//	@Tags			catalog-admin
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Lot UUID"
//	@Param			body	body		dto.ScheduleRequest		false	"Optional scheduledAt (ISO-8601 UTC)"
//	@Success		200		{object}	dto.LotResp
//	@Failure		400		{object}	dto.ErrorResponse	"Weekly cap reached / not certified"
//	@Failure		404		{object}	dto.ErrorResponse	"Lot not found"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/lots/{id}/schedule [post]
func (h *lotHandler) schedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Schedule")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	var req dto.ScheduleRequest
	// Body is optional; ignore EOF on empty body.
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	scheduledAt, err := req.ParseScheduledAt()
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	l, err := h.lot.Schedule(ctx, id, scheduledAt)
	if err != nil {
		logger.WarnContext(ctx, "schedule failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToLotResp(l))
}

// writeJSON encodes v as JSON with the given status, logging encode failures.
func writeJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}
