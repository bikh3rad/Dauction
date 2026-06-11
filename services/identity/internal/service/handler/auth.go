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

type authHandler struct {
	logger *slog.Logger
	mux    *http.ServeMux
	auth   biz.UsecaseAuth
}

var _ service.Handler = (*authHandler)(nil)

// NewAuthHandler constructs the onboarding (auth) HTTP handler.
func NewAuthHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseAuth) *authHandler {
	return &authHandler{
		logger: logger.With("layer", "AuthHandler"),
		mux:    mux,
		auth:   uc,
	}
}

// RegisterHandler self-registers the public onboarding routes.
func (h *authHandler) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("POST /apis/auth/otp/request", h.requestOTP)
	h.mux.HandleFunc("POST /apis/auth/otp/verify", h.verifyOTP)
	h.mux.HandleFunc("GET /apis/auth/oauth/{provider}/callback", h.oauthCallback)
	h.mux.HandleFunc("POST /apis/auth/oauth/{provider}/callback", h.oauthCallback)

	return nil
}

// requestOTP issues a one-time login/signup code.
//
//	@Summary		Request mobile OTP
//	@Description	Issues a one-time code for a mobile number (E.164). In dev mode the code is returned in `devCode`; production delivers it by SMS.
//	@Tags			identity-auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.RequestOTPReq	true	"Mobile + purpose"
//	@Success		202		{object}	dto.RequestOTPResp
//	@Failure		400		{object}	dto.ErrorResponse
//	@Router			/apis/auth/otp/request [post]
func (h *authHandler) requestOTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "RequestOTP")

	var body dto.RequestOTPReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	ttl, devCode, err := h.auth.RequestOTP(ctx, body.MobileE164, body.Purpose)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusAccepted, dto.RequestOTPResp{ExpiresInSecs: ttl, DevCode: devCode})
}

// verifyOTP validates a code and returns a session.
//
//	@Summary		Verify mobile OTP
//	@Description	Validates a code; creates the account on first verify and returns a session token + account.
//	@Tags			identity-auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.VerifyOTPReq	true	"Mobile + code"
//	@Success		200		{object}	dto.SessionResp
//	@Failure		401		{object}	dto.ErrorResponse
//	@Router			/apis/auth/otp/verify [post]
func (h *authHandler) verifyOTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "VerifyOTP")

	var body dto.VerifyOTPReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	res, err := h.auth.VerifyOTP(ctx, body.MobileE164, body.Code)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToSessionResp(res))
}

// oauthCallback exchanges a provider code (dev stub) and returns a session.
//
//	@Summary		OAuth callback
//	@Description	Links/creates an account from a social provider (Google/Facebook/Apple) and returns a session. Dev stub treats the `code` as the provider user id.
//	@Tags			identity-auth
//	@Produce		json
//	@Param			provider	path		string	true	"Provider (google|facebook|apple)"
//	@Param			code		query		string	true	"Provider authorization code"
//	@Success		200			{object}	dto.SessionResp
//	@Failure		400			{object}	dto.ErrorResponse
//	@Router			/apis/auth/oauth/{provider}/callback [get]
func (h *authHandler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "OAuthCallback")

	code := r.URL.Query().Get("code")
	res, err := h.auth.OAuthLogin(ctx, r.PathValue("provider"), code)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToSessionResp(res))
}
