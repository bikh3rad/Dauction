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
// (CLAUDE.md §6).
const accountIDHeader = "X-Account-Id"

type vaultHandler struct {
	logger *slog.Logger
	mux    *http.ServeMux
	uc     biz.UsecaseVault
}

var _ service.Handler = (*vaultHandler)(nil)

// NewVaultHandler constructs the vault HTTP handler.
func NewVaultHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseVault) *vaultHandler {
	return &vaultHandler{
		logger: logger.With("layer", "VaultHandler"),
		mux:    mux,
		uc:     uc,
	}
}

// RegisterHandler self-registers the vault routes (Go 1.22 method patterns).
func (h *vaultHandler) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("GET /apis/vault", h.view)
	h.mux.HandleFunc("POST /apis/vault/objects", h.create)
	h.mux.HandleFunc("POST /apis/vault/objects/{id}/list", h.list)
	h.mux.HandleFunc("POST /apis/vault/objects/{id}/buyback", h.buyback)

	return nil
}

// view returns the caller's objects and Vault-Credit balance.
//
//	@Summary		List my vault
//	@Description	Returns the authenticated member's vault objects plus their derived Vault-Credit balance (USDC cents). Subject from the gateway-injected X-Account-Id header.
//	@Tags			vault
//	@Produce		json
//	@Param			X-Account-Id	header		string	true	"Authenticated account UUID (gateway-injected)"
//	@Success		200				{object}	dto.VaultViewResp
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/vault [get]
func (h *vaultHandler) view(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "View")

	owner, ok := h.caller(w, r, logger)
	if !ok {
		return
	}

	v, err := h.uc.View(ctx, owner)
	if err != nil {
		logger.ErrorContext(ctx, "view failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.VaultViewResp{
		Objects:            dto.ToVaultObjectResps(v.Objects),
		CreditBalanceCents: v.CreditBalanceCents,
	})
}

// create adds a new object to the caller's vault.
//
//	@Summary		Add a vault object
//	@Description	Adds a new IN_VAULT object to the caller's collection. Appraised value is int64 USDC cents.
//	@Tags			vault
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string				true	"Authenticated account UUID (gateway-injected)"
//	@Param			body			body		dto.CreateObjectReq	true	"Object to add"
//	@Success		201				{object}	dto.VaultObjectResp
//	@Failure		400				{object}	dto.ErrorResponse	"Bad Request"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/vault/objects [post]
func (h *vaultHandler) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Create")

	owner, ok := h.caller(w, r, logger)
	if !ok {
		return
	}

	req := new(dto.CreateObjectReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "decode body failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	obj, err := h.uc.AddObject(ctx, owner, req.Title, req.Description, req.AppraisedValueCents)
	if err != nil {
		logger.WarnContext(ctx, "add object failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusCreated, dto.ToVaultObjectResp(obj))
}

// list lists an object to auction.
//
//	@Summary		List an object to auction
//	@Description	Moves an owned IN_VAULT object to APPRAISING and emits object.listed. durationDays is REQUIRED for VICKREY/UNIQBID (2/5/7) and FORBIDDEN for DUTCH.
//	@Tags			vault
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string				true	"Authenticated account UUID (gateway-injected)"
//	@Param			id				path		string				true	"Object UUID"
//	@Param			body			body		dto.ListObjectReq	true	"Auction type + optional duration"
//	@Success		200				{object}	dto.VaultObjectResp
//	@Failure		400				{object}	dto.ErrorResponse	"Invalid mode/duration or state"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated / not owner"
//	@Failure		404				{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/vault/objects/{id}/list [post]
func (h *vaultHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "List")

	owner, ok := h.caller(w, r, logger)
	if !ok {
		return
	}

	objectID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	req := new(dto.ListObjectReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "decode body failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	obj, err := h.uc.List(ctx, owner, objectID, biz.ListRequest{
		Mode:         entity.AuctionMode(req.Atype),
		DurationDays: req.DurationDays,
		CategoryCode: req.CategoryID,
		PrimaryLang:  req.PrimaryLang,
		Translations: req.ToEntityTranslations(),
		ImageRefs:    req.ImageRefs,
	})
	if err != nil {
		logger.WarnContext(ctx, "list object failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToVaultObjectResp(obj))
}

// buyback takes instant buyback on an object.
//
//	@Summary		Instant buyback
//	@Description	Instant buyback on an owned IN_VAULT object. CASH pays 50% of the appraised value in USDC; CREDIT credits 85% to the Vault-Credit ledger (emits credit.changed). Object -> BOUGHT_BACK.
//	@Tags			vault
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string			true	"Authenticated account UUID (gateway-injected)"
//	@Param			id				path		string			true	"Object UUID"
//	@Param			body			body		dto.BuybackReq	true	"Payout mode"
//	@Success		200				{object}	dto.BuybackResp
//	@Failure		400				{object}	dto.ErrorResponse	"Invalid mode or state"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated / not owner"
//	@Failure		404				{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/vault/objects/{id}/buyback [post]
func (h *vaultHandler) buyback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Buyback")

	owner, ok := h.caller(w, r, logger)
	if !ok {
		return
	}

	objectID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	req := new(dto.BuybackReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "decode body failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	res, err := h.uc.Buyback(ctx, owner, objectID, entity.BuybackMode(req.Mode))
	if err != nil {
		logger.WarnContext(ctx, "buyback failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.BuybackResp{
		Object:       dto.ToVaultObjectResp(res.Object),
		Mode:         string(res.Mode),
		PayoutCents:  res.PayoutCents,
		BalanceCents: res.BalanceCents,
	})
}

// caller extracts and validates the authenticated subject from the gateway
// header, writing an ACCESS_DENIED error and returning false on failure.
func (h *vaultHandler) caller(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (uuid.UUID, bool) {
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
