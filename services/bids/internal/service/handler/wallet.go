package handler

import (
	"application/internal/biz"
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
// bids trusts it because the gateway is the only ingress (CLAUDE.md §6).
const headerAccountID = "X-Account-Id"

type walletHandler struct {
	logger *slog.Logger
	mux    *http.ServeMux
	uc     biz.UsecaseWallet
}

var _ service.Handler = (*walletHandler)(nil)

// NewWalletHandler constructs the bids HTTP handler.
func NewWalletHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseWallet) *walletHandler {
	return &walletHandler{
		logger: logger.With("layer", "WalletHandler"),
		mux:    mux,
		uc:     uc,
	}
}

// RegisterHandler self-registers the bids routes (Go 1.22 method patterns).
func (h *walletHandler) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("GET /apis/bids/wallet", h.wallet)
	h.mux.HandleFunc("GET /apis/bids/packages", h.packages)
	h.mux.HandleFunc("POST /apis/bids/buy", h.buy)
	h.mux.HandleFunc("POST /apis/internal/bids/debit", h.debit)

	return nil
}

// wallet returns the caller's read-through balance + recent activity.
//
//	@Summary		Bid-credit wallet
//	@Description	Returns the authenticated account's bid-credit balance (whole credits, $1 each) plus recent purchases and debits. Balance is read-through, never recomputed. Subject from the gateway-injected X-Account-Id header.
//	@Tags			bids
//	@Produce		json
//	@Param			X-Account-Id	header		string	true	"Authenticated account UUID (gateway-injected)"
//	@Success		200				{object}	dto.WalletResp
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/bids/wallet [get]
func (h *walletHandler) wallet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Wallet")

	id, err := uuid.Parse(r.Header.Get(headerAccountID))
	if err != nil {
		logger.WarnContext(ctx, "missing/invalid X-Account-Id", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, err), w)

		return
	}

	view, err := h.uc.Wallet(ctx, id, 0)
	if err != nil {
		logger.ErrorContext(ctx, "get wallet failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToWalletResp(view))
}

// packages lists the purchasable credit packages.
//
//	@Summary		Credit packages
//	@Description	Lists the purchasable bid-credit packages. PUBLIC. Each credit = $1; price is USDC cents.
//	@Tags			bids
//	@Produce		json
//	@Success		200	{array}		dto.PackageResp
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/bids/packages [get]
func (h *walletHandler) packages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Packages")

	pkgs, err := h.uc.Packages(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "list packages failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	out := make([]dto.PackageResp, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, dto.ToPackageResp(p))
	}

	writeJSON(ctx, w, logger, http.StatusOK, out)
}

// buy purchases a credit package.
//
//	@Summary		Buy bid credits
//	@Description	Records a credit-package purchase atomically (the USDC charge + the credit grant). Emits bids.purchased. Idempotent on idempotencyKey. Unknown package -> RESOURCE_INVALID.
//	@Tags			bids
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string				true	"Authenticated account UUID (gateway-injected)"
//	@Param			body			body		dto.BuyBidsRequest	true	"Package id (+ optional idempotency key)"
//	@Success		200				{object}	dto.BuyBidsResp
//	@Failure		400				{object}	dto.ErrorResponse	"Unknown package / bad body"
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/bids/buy [post]
func (h *walletHandler) buy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Buy")

	id, err := uuid.Parse(r.Header.Get(headerAccountID))
	if err != nil {
		logger.WarnContext(ctx, "missing/invalid X-Account-Id", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, err), w)

		return
	}

	req := new(dto.BuyBidsRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "decode body failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	if req.PackageID == "" {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, errors.New("packageId is required")), w)

		return
	}

	res, err := h.uc.Buy(ctx, id, req.PackageID, req.IdempotencyKey)
	if err != nil {
		logger.WarnContext(ctx, "buy failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.BuyBidsResp{
		CreditsGranted: res.CreditsGranted,
		USDCCents:      res.USDCChargedCents,
		Balance:        res.Balance,
	})
}

// debit is the internal idempotent debit-on-bid called by auction-passive.
//
//	@Summary		Debit a bid credit (internal)
//	@Description	Called by auction-passive BEFORE recording a bid. Spends `amount` credits. Insufficient balance -> RESOURCE_INVALID ("out of credits"). Idempotent on idempotencyKey: a replay returns the original debit (HTTP 200, same body), never double-burns. Emits bids.debited.
//	@Tags			bids-internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.DebitRequest	true	"Account, amount, idempotency key, auction"
//	@Success		200		{object}	dto.DebitResultResp
//	@Failure		400		{object}	dto.ErrorResponse	"Out of credits / bad body"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/internal/bids/debit [post]
func (h *walletHandler) debit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Debit")

	req := new(dto.DebitRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "decode body failed", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, errors.New("accountId must be a UUID")), w)

		return
	}

	if req.IdempotencyKey == "" {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, errors.New("idempotencyKey is required")), w)

		return
	}

	res, err := h.uc.Debit(ctx, accountID, req.Amount, req.IdempotencyKey, req.AuctionID)
	if err != nil {
		logger.WarnContext(ctx, "debit failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.DebitResultResp{
		Amount:  res.Amount,
		Balance: res.Balance,
	})
}

// writeJSON encodes v as JSON with the given status, logging encode failures.
func writeJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}
