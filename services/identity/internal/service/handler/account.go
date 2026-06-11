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
// Identity trusts it because the gateway is the only ingress (CLAUDE.md §6).
const headerAccountID = "X-Account-Id"

type accountHandler struct {
	logger  *slog.Logger
	mux     *http.ServeMux
	account biz.UsecaseAccount
}

var _ service.Handler = (*accountHandler)(nil)

// NewAccountHandler constructs the account HTTP handler.
func NewAccountHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseAccount) *accountHandler {
	return &accountHandler{
		logger:  logger.With("layer", "AccountHandler"),
		mux:     mux,
		account: uc,
	}
}

// RegisterHandler self-registers the identity routes (Go 1.22 method patterns).
func (h *accountHandler) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("GET /apis/me", h.me)
	h.mux.HandleFunc("GET /apis/internal/accounts/{id}/access", h.access)
	h.mux.HandleFunc("POST /apis/admin/accounts/{id}/vip", h.grantVIP)

	// Admin user management (CLAUDE.md §6 admin; behind the gateway admin guard).
	h.mux.HandleFunc("GET /apis/admin/users", h.listUsers)
	h.mux.HandleFunc("GET /apis/admin/users/{id}", h.getUser)
	h.mux.HandleFunc("PATCH /apis/admin/users/{id}", h.updateUser)
	h.mux.HandleFunc("POST /apis/admin/users/{id}/roles", h.assignRole)
	h.mux.HandleFunc("DELETE /apis/admin/users/{id}/roles/{role}", h.revokeRole)

	return nil
}

// me returns the current account (tier, KYC status, eligibility).
//
//	@Summary		Current account
//	@Description	Returns the authenticated account's tier, KYC status and participation eligibility. The gateway injects the subject via the X-Account-Id header.
//	@Tags			identity
//	@Produce		json
//	@Param			X-Account-Id	header		string	true	"Authenticated account UUID (gateway-injected)"
//	@Success		200				{object}	dto.AccountResp
//	@Failure		401				{object}	dto.ErrorResponse	"Unauthenticated"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/me [get]
func (h *accountHandler) me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Me")

	id, err := uuid.Parse(r.Header.Get(headerAccountID))
	if err != nil {
		logger.WarnContext(ctx, "missing/invalid X-Account-Id", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, err), w)

		return
	}

	acc, err := h.account.Get(ctx, id)
	if err != nil {
		logger.ErrorContext(ctx, "get account failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAccountResp(acc))
}

// access is the internal tier+KYC read for the gateway guard.
//
//	@Summary		Internal access read
//	@Description	Minimal tier + KYC + eligibility read model for the gateway's authorization guard. Not exposed to end users.
//	@Tags			identity-internal
//	@Produce		json
//	@Param			id	path		string	true	"Account UUID"
//	@Success		200	{object}	dto.AccessResp
//	@Failure		400	{object}	dto.ErrorResponse	"Bad Request"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/internal/accounts/{id}/access [get]
func (h *accountHandler) access(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Access")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		logger.WarnContext(ctx, "invalid account UUID", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	acc, err := h.account.Get(ctx, id)
	if err != nil {
		logger.ErrorContext(ctx, "get account failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAccessResp(acc))
}

// grantVIP is the house/admin VIP grant.
//
//	@Summary		Grant VIP
//	@Description	House/admin action that elevates an account to VIP. Tier only rises; a no-op or downgrade returns RESOURCE_INVALID. Emits account.tier_changed.
//	@Tags			identity-admin
//	@Produce		json
//	@Param			id	path		string	true	"Account UUID"
//	@Success		200	{object}	dto.AccountResp
//	@Failure		400	{object}	dto.ErrorResponse	"Invalid transition / bad UUID"
//	@Failure		404	{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/accounts/{id}/vip [post]
func (h *accountHandler) grantVIP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "GrantVIP")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		logger.WarnContext(ctx, "invalid account UUID", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	acc, err := h.account.GrantVIP(ctx, id)
	if err != nil {
		logger.ErrorContext(ctx, "grant VIP failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAccountResp(acc))
}

// writeJSON encodes v as JSON with the given status, logging encode failures.
func writeJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}
