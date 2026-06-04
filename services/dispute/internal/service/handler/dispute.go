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

	"github.com/google/uuid"
)

// headerAccountID is the authenticated subject the gateway injects after authN.
// headerAdmin marks a house/admin caller (gateway sets it for admin sessions).
const (
	headerAccountID = "X-Account-Id"
	headerAdmin     = "X-Admin"
)

type disputeHandler struct {
	logger  *slog.Logger
	mux     *http.ServeMux
	dispute biz.UsecaseDispute
}

var _ service.Handler = (*disputeHandler)(nil)

// NewDisputeHandler constructs the dispute HTTP handler.
func NewDisputeHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseDispute) *disputeHandler {
	return &disputeHandler{
		logger:  logger.With("layer", "DisputeHandler"),
		mux:     mux,
		dispute: uc,
	}
}

// RegisterHandler self-registers the dispute routes (Go 1.22 method patterns).
func (h *disputeHandler) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("POST /apis/escrow/{tradeId}/dispute", h.open)
	h.mux.HandleFunc("GET /apis/escrow/{tradeId}/dispute", h.get)
	h.mux.HandleFunc("POST /apis/escrow/{tradeId}/dispute/evidence", h.addEvidence)
	h.mux.HandleFunc("POST /apis/escrow/{tradeId}/dispute/resolve", h.resolve)
	h.mux.HandleFunc("POST /apis/escrow/{tradeId}/dispute/withdraw", h.withdraw)
	h.mux.HandleFunc("GET /apis/admin/disputes", h.adminList)
	h.mux.HandleFunc("POST /apis/admin/disputes/{id}/review", h.adminReview)

	return nil
}

// open lets a buyer raise a dispute on a delivered trade.
//
//	@Summary		Open a dispute
//	@Description	A buyer (claimant) raises an authenticity/condition/delivery claim on a trade post-delivery. Creates an OPEN dispute + audit row and emits dispute.opened (escrow suspends release: HELD -> DISPUTED). Only one non-terminal dispute may exist per trade.
//	@Tags			dispute
//	@Accept			json
//	@Produce		json
//	@Param			tradeId			path		string				true	"Escrow trade / auction id"
//	@Param			X-Account-Id	header		string				true	"Claimant (buyer) account UUID (gateway-injected)"
//	@Param			body			body		dto.OpenDisputeReq	true	"Dispute reason + respondent"
//	@Success		201				{object}	dto.DisputeResp
//	@Failure		400				{object}	dto.ErrorResponse	"Invalid body / reason"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated"
//	@Failure		409				{object}	dto.ErrorResponse	"A non-terminal dispute already exists"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/escrow/{tradeId}/dispute [post]
func (h *disputeHandler) open(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Open")

	caller, ok := h.caller(w, r)
	if !ok {
		return
	}

	var req dto.OpenDisputeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.WarnContext(ctx, "decode failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	respondent, err := uuid.Parse(req.Respondent)
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	d, err := h.dispute.Open(ctx, biz.OpenParams{
		TradeID:     r.PathValue("tradeId"),
		Claimant:    caller,
		Respondent:  respondent,
		ReasonCode:  entity.ReasonCode(req.ReasonCode),
		EvidenceRef: req.EvidenceRef,
	})
	if err != nil {
		logger.WarnContext(ctx, "open dispute failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusCreated, dto.ToDisputeResp(d))
}

// get returns the dispute + audit trail for a trade (parties or admin only).
//
//	@Summary		Get a dispute
//	@Description	Returns the current dispute for a trade plus its immutable audit trail. Only the claimant, the respondent, or an admin (X-Admin: true) may read it.
//	@Tags			dispute
//	@Produce		json
//	@Param			tradeId			path		string	true	"Escrow trade / auction id"
//	@Param			X-Account-Id	header		string	true	"Caller account UUID (gateway-injected)"
//	@Param			X-Admin			header		string	false	"Set to 'true' for house/admin access"
//	@Success		200				{object}	dto.DisputeDetailResp
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated / not a party"
//	@Failure		404				{object}	dto.ErrorResponse	"No dispute for trade"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/escrow/{tradeId}/dispute [get]
func (h *disputeHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Get")

	caller, ok := h.caller(w, r)
	if !ok {
		return
	}

	view, err := h.dispute.Get(ctx, r.PathValue("tradeId"), caller, isAdmin(r))
	if err != nil {
		logger.WarnContext(ctx, "get dispute failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.DisputeDetailResp{
		Dispute: dto.ToDisputeResp(view.Dispute),
		Events:  dto.ToDisputeEventResps(view.Events),
	})
}

// addEvidence appends an evidence audit row (either party).
//
//	@Summary		Add evidence
//	@Description	Either party (claimant or respondent) appends an evidence reference to a non-terminal dispute. Records an EVIDENCE_ADDED audit row.
//	@Tags			dispute
//	@Accept			json
//	@Produce		json
//	@Param			tradeId			path	string				true	"Escrow trade / auction id"
//	@Param			X-Account-Id	header	string				true	"Caller account UUID (gateway-injected)"
//	@Param			body			body	dto.AddEvidenceReq	true	"Evidence reference"
//	@Success		204				""
//	@Failure		400				{object}	dto.ErrorResponse	"Invalid body / terminal dispute"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated / not a party"
//	@Failure		404				{object}	dto.ErrorResponse	"No dispute for trade"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/escrow/{tradeId}/dispute/evidence [post]
func (h *disputeHandler) addEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "AddEvidence")

	caller, ok := h.caller(w, r)
	if !ok {
		return
	}

	var req dto.AddEvidenceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DetailRef == "" {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, errors.New("detailRef is required")), w)

		return
	}

	if err := h.dispute.AddEvidence(ctx, r.PathValue("tradeId"), caller, req.DetailRef); err != nil {
		logger.WarnContext(ctx, "add evidence failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// resolve is the house/admin ruling.
//
//	@Summary		Resolve a dispute
//	@Description	House/admin rules on a dispute that is UNDER_REVIEW, moving it to RESOLVED and emitting dispute.resolved {trade_id, ruling} (escrow executes the funds movement). The ruling is immutable once set.
//	@Tags			dispute-admin
//	@Accept			json
//	@Produce		json
//	@Param			tradeId	path		string			true	"Escrow trade / auction id"
//	@Param			body	body		dto.ResolveReq	true	"Ruling + ruledBy"
//	@Success		200		{object}	dto.DisputeResp
//	@Failure		400		{object}	dto.ErrorResponse	"Invalid ruling / not UNDER_REVIEW / already resolved"
//	@Failure		404		{object}	dto.ErrorResponse	"No dispute for trade"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/escrow/{tradeId}/dispute/resolve [post]
func (h *disputeHandler) resolve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Resolve")

	var req dto.ResolveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	ruledBy, err := uuid.Parse(req.RuledBy)
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	d, err := h.dispute.Resolve(ctx, r.PathValue("tradeId"), entity.Ruling(req.Ruling), ruledBy)
	if err != nil {
		logger.WarnContext(ctx, "resolve failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToDisputeResp(d))
}

// withdraw lets the claimant retract an OPEN dispute.
//
//	@Summary		Withdraw a dispute
//	@Description	The claimant retracts a dispute that is still OPEN, moving it to WITHDRAWN. Only the claimant may withdraw, and only from OPEN.
//	@Tags			dispute
//	@Produce		json
//	@Param			tradeId			path		string	true	"Escrow trade / auction id"
//	@Param			X-Account-Id	header		string	true	"Claimant account UUID (gateway-injected)"
//	@Success		200				{object}	dto.DisputeResp
//	@Failure		400				{object}	dto.ErrorResponse	"Not OPEN"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated / not the claimant"
//	@Failure		404				{object}	dto.ErrorResponse	"No dispute for trade"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/escrow/{tradeId}/dispute/withdraw [post]
func (h *disputeHandler) withdraw(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Withdraw")

	caller, ok := h.caller(w, r)
	if !ok {
		return
	}

	d, err := h.dispute.Withdraw(ctx, r.PathValue("tradeId"), caller)
	if err != nil {
		logger.WarnContext(ctx, "withdraw failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToDisputeResp(d))
}

// adminList is the dispute-court queue.
//
//	@Summary		List disputes (admin queue)
//	@Description	House/admin queue of disputes, optionally filtered by state (?state=OPEN|UNDER_REVIEW|RESOLVED|WITHDRAWN), newest first.
//	@Tags			dispute-admin
//	@Produce		json
//	@Param			state	query		string	false	"Filter by state"
//	@Success		200		{object}	dto.DisputeListResp
//	@Failure		400		{object}	dto.ErrorResponse	"Unknown state filter"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/disputes [get]
func (h *disputeHandler) adminList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "AdminList")

	ds, err := h.dispute.ListByState(ctx, biz.ListFilter{State: entity.State(r.URL.Query().Get("state"))})
	if err != nil {
		logger.WarnContext(ctx, "list disputes failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToDisputeListResp(ds))
}

// adminReview moves a dispute OPEN -> UNDER_REVIEW.
//
//	@Summary		Start review
//	@Description	House/admin moves a dispute from OPEN to UNDER_REVIEW (triage before a ruling). Records a REVIEW_STARTED audit row.
//	@Tags			dispute-admin
//	@Produce		json
//	@Param			id				path		string	true	"Dispute UUID"
//	@Param			X-Account-Id	header		string	true	"Admin account UUID (gateway-injected)"
//	@Success		200				{object}	dto.DisputeResp
//	@Failure		400				{object}	dto.ErrorResponse	"Not OPEN / bad UUID"
//	@Failure		404				{object}	dto.ErrorResponse	"No such dispute"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/disputes/{id}/review [post]
func (h *disputeHandler) adminReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "AdminReview")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	// the admin actor is recorded in the audit trail; fall back to nil if absent.
	admin, _ := uuid.Parse(r.Header.Get(headerAccountID))

	d, err := h.dispute.StartReview(ctx, id, admin)
	if err != nil {
		logger.WarnContext(ctx, "start review failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToDisputeResp(d))
}

// caller extracts the authenticated subject from X-Account-Id, writing a 401 and
// returning ok=false when it is missing/invalid.
func (h *disputeHandler) caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.Header.Get(headerAccountID))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, err), w)

		return uuid.Nil, false
	}

	return id, true
}

// isAdmin reports whether the gateway flagged this as a house/admin call.
func isAdmin(r *http.Request) bool {
	return r.Header.Get(headerAdmin) == "true"
}

// writeJSON encodes v as JSON with the given status, logging encode failures.
func writeJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}
