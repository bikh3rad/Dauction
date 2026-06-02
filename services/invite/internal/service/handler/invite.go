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
)

// accountIDHeader carries the authenticated account id forwarded by the gateway
// after authN + tier/KYC guard. Handlers treat it as the trusted caller identity.
const accountIDHeader = "X-Account-Id"

type invite struct {
	logger *slog.Logger
	mux    *http.ServeMux
	uc     biz.UsecaseInvite
}

var _ service.Handler = (*invite)(nil)

// NewInvite creates the invite HTTP handler.
func NewInvite(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseInvite) *invite {
	return &invite{
		logger: logger.With("layer", "InviteHandler"),
		mux:    mux,
		uc:     uc,
	}
}

// RegisterHandler self-registers the invite routes (Go 1.22 method patterns).
func (h *invite) RegisterHandler(_ context.Context) error {
	// member/VIP surface
	h.mux.HandleFunc("POST /apis/invites/redeem", h.redeem)
	h.mux.HandleFunc("POST /apis/invites", h.issue)
	// admin surface
	h.mux.HandleFunc("GET /apis/admin/invites", h.adminList)
	h.mux.HandleFunc("POST /apis/admin/invites/{code}/revoke", h.adminRevoke)
	h.mux.HandleFunc("POST /apis/admin/invites/{code}/flag", h.adminFlag)
	h.mux.HandleFunc("GET /apis/admin/invites/chain/{accountId}", h.adminChain)

	return nil
}

// redeem consumes a single-use invite code for the authenticated account.
//
//	@Summary		Redeem an invite code
//	@Description	Atomically consume a single-use invite code, elevating the caller. Emits invite.redeemed.
//	@Tags			Invites
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string				true	"Authenticated account id (gateway-injected)"
//	@Param			body			body		dto.RedeemInviteReq	true	"Invite code"
//	@Success		200				{object}	dto.RedeemInviteResp
//	@Failure		400				{object}	dto.ErrorResponse	"Bad Request / not redeemable"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/invites/redeem [post]
func (h *invite) redeem(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With("method", "Redeem")
	ctx := r.Context()

	accountID := r.Header.Get(accountIDHeader)
	if accountID == "" {
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, errors.New("missing account id")), w)

		return
	}

	req := new(dto.RedeemInviteReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		logger.WarnContext(ctx, "failed to decode body", "error", err)
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	res, err := h.uc.Redeem(ctx, req.Code, accountID)
	if err != nil {
		logger.WarnContext(ctx, "redeem failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(w, http.StatusOK, dto.RedeemInviteResp{
		Code:       res.Code,
		RedeemedBy: res.RedeemedBy,
		IssuedBy:   res.IssuerAccountID,
	}, logger, ctx)
}

// issue creates a new single-use invite code owned by the caller.
//
//	@Summary		Issue an invite code
//	@Description	A MEMBER/VIP issues a new single-use invite code (subject to the per-issuer quota).
//	@Tags			Invites
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string	true	"Authenticated account id (gateway-injected)"
//	@Success		201				{object}	dto.IssueInviteResp
//	@Failure		400				{object}	dto.ErrorResponse	"Quota exceeded / invalid"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/invites [post]
func (h *invite) issue(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With("method", "Issue")
	ctx := r.Context()

	accountID := r.Header.Get(accountIDHeader)
	if accountID == "" {
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, errors.New("missing account id")), w)

		return
	}

	inv, err := h.uc.Issue(ctx, accountID)
	if err != nil {
		logger.WarnContext(ctx, "issue failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(w, http.StatusCreated, dto.IssueInviteResp{
		ID:              inv.ID.String(),
		Code:            inv.Code,
		IssuerAccountID: inv.IssuerAccountID,
		Status:          string(inv.Status),
		CreatedAt:       inv.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}, logger, ctx)
}

// adminList lists invites with optional status/issuer filtering.
//
//	@Summary		List invites (admin)
//	@Description	List/filter invites for the admin console.
//	@Tags			Invites Admin
//	@Produce		json
//	@Param			status	query		string	false	"Filter by status (ISSUED|REDEEMED|REVOKED|FLAGGED)"
//	@Param			issuer	query		string	false	"Filter by issuer account id"
//	@Param			limit	query		int		false	"Page size (default 50, max 200)"
//	@Param			offset	query		int		false	"Page offset"
//	@Success		200		{object}	dto.InviteListResp
//	@Failure		400		{object}	dto.ErrorResponse	"Bad Request"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/invites [get]
func (h *invite) adminList(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With("method", "AdminList")
	ctx := r.Context()

	f := biz.ListInvitesFilter{
		Status:          r.URL.Query().Get("status"),
		IssuerAccountID: r.URL.Query().Get("issuer"),
		Limit:           atoiDefault(r.URL.Query().Get("limit"), 0),
		Offset:          atoiDefault(r.URL.Query().Get("offset"), 0),
	}

	invites, err := h.uc.List(ctx, f)
	if err != nil {
		logger.WarnContext(ctx, "list failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(w, http.StatusOK, dto.ToInviteListResp(invites), logger, ctx)
}

// adminRevoke revokes an ISSUED invite code.
//
//	@Summary		Revoke an invite (admin)
//	@Description	Move an ISSUED code to REVOKED. Non-ISSUED -> 400.
//	@Tags			Invites Admin
//	@Produce		json
//	@Param			code	path	string	true	"Invite code"
//	@Success		204		""
//	@Failure		400		{object}	dto.ErrorResponse	"Not in ISSUED state"
//	@Failure		404		{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/invites/{code}/revoke [post]
func (h *invite) adminRevoke(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With("method", "AdminRevoke")
	ctx := r.Context()

	if err := h.uc.Revoke(ctx, r.PathValue("code")); err != nil {
		logger.WarnContext(ctx, "revoke failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// adminFlag flags an ISSUED invite code for review.
//
//	@Summary		Flag an invite (admin)
//	@Description	Move an ISSUED code to FLAGGED. Non-ISSUED -> 400.
//	@Tags			Invites Admin
//	@Produce		json
//	@Param			code	path	string	true	"Invite code"
//	@Success		204		""
//	@Failure		400		{object}	dto.ErrorResponse	"Not in ISSUED state"
//	@Failure		404		{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500		{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/invites/{code}/flag [post]
func (h *invite) adminFlag(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With("method", "AdminFlag")
	ctx := r.Context()

	if err := h.uc.Flag(ctx, r.PathValue("code")); err != nil {
		logger.WarnContext(ctx, "flag failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// adminChain returns the invite chain rooted at an account.
//
//	@Summary		Invite chain (admin)
//	@Description	List the invitees an account brought in (invite_edge where it is the inviter).
//	@Tags			Invites Admin
//	@Produce		json
//	@Param			accountId	path		string	true	"Inviter account id"
//	@Success		200			{object}	dto.InviteChainResp
//	@Failure		400			{object}	dto.ErrorResponse	"Bad Request"
//	@Failure		500			{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/invites/chain/{accountId} [get]
func (h *invite) adminChain(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With("method", "AdminChain")
	ctx := r.Context()

	accountID := r.PathValue("accountId")
	edges, err := h.uc.Chain(ctx, accountID)
	if err != nil {
		logger.WarnContext(ctx, "chain failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(w, http.StatusOK, dto.ToInviteChainResp(accountID, edges), logger, ctx)
}
