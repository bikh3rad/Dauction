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

// accountIDHeader carries the authenticated account id forwarded by the gateway
// after authN + tier/KYC guard. Handlers treat it as the trusted caller identity
// (root CLAUDE.md §6).
const accountIDHeader = "X-Account-Id"

type escrowHandler struct {
	logger *slog.Logger
	mux    *http.ServeMux
	uc     biz.UsecaseEscrow
}

var _ service.Handler = (*escrowHandler)(nil)

// NewEscrowHandler constructs the escrow HTTP handler.
func NewEscrowHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseEscrow) *escrowHandler {
	return &escrowHandler{
		logger: logger.With("layer", "EscrowHandler"),
		mux:    mux,
		uc:     uc,
	}
}

// RegisterHandler self-registers the escrow routes (Go 1.22 method patterns).
// Lock requests (reservation / full-lock) arrive as escrow.lock_requested events,
// not via HTTP — escrow is the sole funds authority and locks are driven by the
// auction engines through the bus (root §2). No internal lock route is exposed.
func (h *escrowHandler) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("GET /apis/escrow/{tradeId}", h.get)
	h.mux.HandleFunc("POST /apis/escrow/{tradeId}/fund", h.fund)
	h.mux.HandleFunc("POST /apis/escrow/{tradeId}/confirm", h.confirm)
	h.mux.HandleFunc("POST /apis/admin/escrow/{tradeId}/refund", h.refund)
	h.mux.HandleFunc("POST /apis/admin/escrow/{tradeId}/forfeit", h.forfeit)

	return nil
}

// get returns a trade's state + derived per-participant balances + conservation.
//
//	@Summary		Get escrow trade
//	@Description	Returns the escrow trade head state, the derived per-participant balances (USDC cents), and the funds-conservation summary (inflows vs disbursed).
//	@Tags			escrow
//	@Produce		json
//	@Param			tradeId	path		string	true	"Trade UUID (== auction id)"
//	@Success		200		{object}	dto.TradeResp
//	@Failure		400		{object}	dto.ErrorResponse	"Bad Request"
//	@Failure		404		{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/escrow/{tradeId} [get]
func (h *escrowHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Get")

	tradeID, ok := h.tradeID(w, r, logger)
	if !ok {
		return
	}

	view, err := h.uc.Get(ctx, tradeID)
	if err != nil {
		logger.WarnContext(ctx, "get trade failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToTradeResp(view))
}

// fund records the winner funding their obligation.
//
//	@Summary		Fund obligation
//	@Description	The winner funds their obligation (passive path, or Dutch post-hammer). Within the funding deadline the trade moves to HELD; past the deadline it FORFEITs. amountCents must equal the obligation exactly (mismatch -> RESOURCE_INVALID). Double-fund is rejected.
//	@Tags			escrow
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string			true	"Authenticated account UUID (gateway-injected)"
//	@Param			tradeId			path		string			true	"Trade UUID"
//	@Param			body			body		dto.FundReq		true	"Amount to fund (must equal obligation)"
//	@Success		200				{object}	dto.TradeStateResp
//	@Failure		400				{object}	dto.ErrorResponse	"Wrong amount / window expired / already funded"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated / not the winner"
//	@Failure		404				{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/escrow/{tradeId}/fund [post]
func (h *escrowHandler) fund(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Fund")

	tradeID, ok := h.tradeID(w, r, logger)
	if !ok {
		return
	}

	caller, ok := h.caller(w, r, logger)
	if !ok {
		return
	}

	req := new(dto.FundReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "decode body failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	trade, err := h.uc.Fund(ctx, tradeID, caller, req.AmountCents)
	if err != nil {
		logger.WarnContext(ctx, "fund failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToTradeStateResp(trade))
}

// confirm records buyer delivery confirmation -> RELEASED.
//
//	@Summary		Confirm delivery (release)
//	@Description	The buyer confirms delivery, releasing the held pot to the seller (HELD -> RELEASED). mode CASH pays 100%; VAULT_CREDIT records a 110% Vault-Credit instruction (vault credits on escrow.released). Blocked while DISPUTED.
//	@Tags			escrow
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string			true	"Authenticated account UUID (gateway-injected)"
//	@Param			tradeId			path		string			true	"Trade UUID"
//	@Param			body			body		dto.ConfirmReq	true	"Release mode"
//	@Success		200				{object}	dto.TradeStateResp
//	@Failure		400				{object}	dto.ErrorResponse	"Not HELD / disputed / bad mode"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated / not the buyer"
//	@Failure		404				{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/escrow/{tradeId}/confirm [post]
func (h *escrowHandler) confirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Confirm")

	tradeID, ok := h.tradeID(w, r, logger)
	if !ok {
		return
	}

	caller, ok := h.caller(w, r, logger)
	if !ok {
		return
	}

	req := new(dto.ConfirmReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "decode body failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	trade, err := h.uc.Confirm(ctx, tradeID, caller, entity.ReleaseMode(req.Mode))
	if err != nil {
		logger.WarnContext(ctx, "confirm failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToTradeStateResp(trade))
}

// refund is the admin/loser refund.
//
//	@Summary		Refund a participant (admin)
//	@Description	House/admin action: returns a participant's locked funds (loser unfreeze or manual correction) -> REFUNDED. Suspended while DISPUTED.
//	@Tags			escrow-admin
//	@Accept			json
//	@Produce		json
//	@Param			tradeId	path		string			true	"Trade UUID"
//	@Param			body	body		dto.RefundReq	true	"Participant to refund"
//	@Success		200		{object}	dto.TradeStateResp
//	@Failure		400		{object}	dto.ErrorResponse	"Nothing to refund / disputed / bad id"
//	@Failure		404		{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/escrow/{tradeId}/refund [post]
func (h *escrowHandler) refund(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Refund")

	tradeID, ok := h.tradeID(w, r, logger)
	if !ok {
		return
	}

	req := new(dto.RefundReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "decode body failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	participant, err := uuid.Parse(req.ParticipantID)
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	trade, err := h.uc.Refund(ctx, tradeID, participant)
	if err != nil {
		logger.WarnContext(ctx, "refund failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToTradeStateResp(trade))
}

// forfeit is the admin/manual forfeit.
//
//	@Summary		Forfeit a trade (admin)
//	@Description	House/admin action: a winner who missed the 24h funding window forfeits their locked funds -> FORFEITED. Emits escrow.forfeited.
//	@Tags			escrow-admin
//	@Produce		json
//	@Param			tradeId	path		string	true	"Trade UUID"
//	@Success		200		{object}	dto.TradeStateResp
//	@Failure		400		{object}	dto.ErrorResponse	"Nothing to forfeit / disputed"
//	@Failure		404		{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/escrow/{tradeId}/forfeit [post]
func (h *escrowHandler) forfeit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Forfeit")

	tradeID, ok := h.tradeID(w, r, logger)
	if !ok {
		return
	}

	trade, err := h.uc.Forfeit(ctx, tradeID)
	if err != nil {
		logger.WarnContext(ctx, "forfeit failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToTradeStateResp(trade))
}

// tradeID extracts and validates the {tradeId} path value.
func (h *escrowHandler) tradeID(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("tradeId"))
	if err != nil {
		logger.WarnContext(r.Context(), "invalid trade UUID", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return uuid.Nil, false
	}

	return id, true
}

// caller extracts and validates the authenticated subject from the gateway
// header, writing an ACCESS_DENIED error and returning false on failure.
func (h *escrowHandler) caller(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.Header.Get(accountIDHeader))
	if err != nil {
		logger.WarnContext(r.Context(), "missing/invalid X-Account-Id", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, err), w)

		return uuid.Nil, false
	}

	return id, true
}

// writeJSON encodes v as JSON with the given status, logging encode failures.
func writeJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}
