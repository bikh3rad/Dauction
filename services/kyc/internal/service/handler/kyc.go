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

// accountHeader is the header the gateway sets after authenticating the caller.
const accountHeader = "X-Account-Id"

type kyc struct {
	logger *slog.Logger
	mux    *http.ServeMux
	uc     biz.UsecaseKyc
}

var _ service.Handler = (*kyc)(nil)

// NewKyc builds the KYC HTTP handler. The OutboxRelayMarker parameter exists
// only to pull the relay registration into the Wire graph.
func NewKyc(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseKyc, _ biz.OutboxRelayMarker) *kyc {
	return &kyc{
		logger: logger.With("layer", "KycHandler"),
		mux:    mux,
		uc:     uc,
	}
}

// RegisterHandler registers KYC routes.
func (h *kyc) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("POST /apis/kyc/start", h.start)
	h.mux.HandleFunc("POST /apis/kyc/verify", h.verify)
	h.mux.HandleFunc("GET /apis/kyc/status", h.status)

	h.mux.HandleFunc("GET /apis/admin/kyc", h.adminQueue)
	h.mux.HandleFunc("POST /apis/admin/kyc/{id}/approve", h.adminApprove)
	h.mux.HandleFunc("POST /apis/admin/kyc/{id}/reject", h.adminReject)

	return nil
}

// start begins a KYC submission and issues an OTP challenge.
//
//	@Summary		Start KYC
//	@Description	Begin KYC: register a document reference and phone, issuing an OTP challenge.
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string			true	"Authenticated account UUID (set by gateway)"
//	@Param			body			body		dto.StartKycReq	true	"Start KYC request"
//	@Success		201				{object}	dto.StartKycResp
//	@Failure		400				{object}	dto.ErrorResponse
//	@Failure		409				{object}	dto.ErrorResponse
//	@Failure		500				{object}	dto.ErrorResponse
//	@Router			/apis/kyc/start [post]
func (h *kyc) start(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "start")

	accountID, err := accountFromRequest(r)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	req := new(dto.StartKycReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	if req.DocRef == "" || req.Phone == "" || !entity.DocType(req.DocType).Valid() {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, errors.New("missing or invalid fields")), w)

		return
	}

	res, err := h.uc.Start(ctx, biz.StartParams{
		AccountID: accountID,
		DocType:   entity.DocType(req.DocType),
		DocRef:    req.DocRef,
		Phone:     req.Phone,
	})
	if err != nil {
		logger.WarnContext(ctx, "start failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, logger, w, http.StatusCreated, dto.StartKycResp{
		SubmissionID: res.SubmissionID.String(),
		ChallengeID:  res.ChallengeID.String(),
		State:        string(entity.SubmissionStarted),
		ExpiresAt:    res.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		DevCode:      res.DevCode,
	})
}

// verify checks an OTP code and moves the submission into the admin queue.
//
//	@Summary		Verify KYC OTP
//	@Description	Verify the OTP code; on success the submission enters the admin queue (SUBMITTED).
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string				true	"Authenticated account UUID (set by gateway)"
//	@Param			body			body		dto.VerifyKycReq	true	"Verify OTP request"
//	@Success		200				{object}	dto.SubmissionResp
//	@Failure		400				{object}	dto.ErrorResponse
//	@Failure		404				{object}	dto.ErrorResponse
//	@Failure		500				{object}	dto.ErrorResponse
//	@Router			/apis/kyc/verify [post]
func (h *kyc) verify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "verify")

	accountID, err := accountFromRequest(r)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	req := new(dto.VerifyKycReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	if len(req.Code) != 6 {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, errors.New("code must be 6 digits")), w)

		return
	}

	sub, err := h.uc.Verify(ctx, accountID, req.Code)
	if err != nil {
		logger.WarnContext(ctx, "verify failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, logger, w, http.StatusOK, dto.ToSubmissionResp(&sub))
}

// status returns the caller's latest submission.
//
//	@Summary		KYC status
//	@Description	Return the caller's current KYC submission/state.
//	@Tags			KYC
//	@Produce		json
//	@Param			X-Account-Id	header		string	true	"Authenticated account UUID (set by gateway)"
//	@Success		200				{object}	dto.SubmissionResp
//	@Failure		404				{object}	dto.ErrorResponse
//	@Failure		500				{object}	dto.ErrorResponse
//	@Router			/apis/kyc/status [get]
func (h *kyc) status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "status")

	accountID, err := accountFromRequest(r)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	sub, err := h.uc.Status(ctx, accountID)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, logger, w, http.StatusOK, dto.ToSubmissionResp(&sub))
}

// adminQueue lists submissions awaiting a decision.
//
//	@Summary		Admin KYC queue
//	@Description	List submissions awaiting an admin decision (SUBMITTED).
//	@Tags			KYC Admin
//	@Produce		json
//	@Success		200	{object}	dto.SubmissionListResp
//	@Failure		500	{object}	dto.ErrorResponse
//	@Router			/apis/admin/kyc [get]
func (h *kyc) adminQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "adminQueue")

	subs, err := h.uc.PendingQueue(ctx)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, logger, w, http.StatusOK, dto.ToSubmissionResps(subs))
}

// adminApprove approves a submission.
//
//	@Summary		Approve KYC
//	@Description	Approve a submitted KYC; emits kyc.approved.
//	@Tags			KYC Admin
//	@Produce		json
//	@Param			X-Account-Id	header		string	true	"Admin account UUID (set by gateway)"
//	@Param			id				path		string	true	"Submission UUID"
//	@Success		200				{object}	dto.SubmissionResp
//	@Failure		400				{object}	dto.ErrorResponse
//	@Failure		404				{object}	dto.ErrorResponse
//	@Failure		500				{object}	dto.ErrorResponse
//	@Router			/apis/admin/kyc/{id}/approve [post]
func (h *kyc) adminApprove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "adminApprove")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	decidedBy := adminFromRequest(r)

	sub, err := h.uc.Approve(ctx, id, decidedBy)
	if err != nil {
		logger.WarnContext(ctx, "approve failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, logger, w, http.StatusOK, dto.ToSubmissionResp(&sub))
}

// adminReject rejects a submission with a reason code.
//
//	@Summary		Reject KYC
//	@Description	Reject a submitted KYC with a machine reason code; emits kyc.rejected.
//	@Tags			KYC Admin
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string			true	"Admin account UUID (set by gateway)"
//	@Param			id				path		string			true	"Submission UUID"
//	@Param			body			body		dto.RejectKycReq	true	"Rejection reason"
//	@Success		200				{object}	dto.SubmissionResp
//	@Failure		400				{object}	dto.ErrorResponse
//	@Failure		404				{object}	dto.ErrorResponse
//	@Failure		500				{object}	dto.ErrorResponse
//	@Router			/apis/admin/kyc/{id}/reject [post]
func (h *kyc) adminReject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "adminReject")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	req := new(dto.RejectKycReq)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	if req.Reason == "" {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, errors.New("reason required")), w)

		return
	}

	sub, err := h.uc.Reject(ctx, biz.RejectParams{
		SubmissionID: id,
		DecidedBy:    adminFromRequest(r),
		Reason:       req.Reason,
	})
	if err != nil {
		logger.WarnContext(ctx, "reject failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, logger, w, http.StatusOK, dto.ToSubmissionResp(&sub))
}

// accountFromRequest extracts the gateway-set account UUID; absent/invalid is
// access-denied.
func accountFromRequest(r *http.Request) (uuid.UUID, error) {
	raw := r.Header.Get(accountHeader)
	if raw == "" {
		return uuid.Nil, errors.Join(biz.ErrResourceAccessDenied, errors.New("missing account header"))
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, errors.Join(biz.ErrResourceAccessDenied, errors.New("invalid account header"))
	}

	return id, nil
}

// adminFromRequest returns the admin account if present, else Nil (the gateway
// enforces the admin role).
func adminFromRequest(r *http.Request) uuid.UUID {
	if id, err := accountFromRequest(r); err == nil {
		return id
	}

	return uuid.Nil
}

func writeJSON(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.ErrorContext(ctx, "encode response failed", "error", err)
	}
}
