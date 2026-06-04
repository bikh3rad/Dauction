package dto

import (
	"application/internal/biz"
	"encoding/json"
	"errors"
	"net/http"
)

// ErrorResponse is the language-neutral error envelope. `code` is a stable
// MONOSPACE enum string the React client localizes; `message` is a short English
// developer hint (never shown to end users). Integer amounts / ISO timestamps and
// all display copy live client-side (root CLAUDE.md §0.7).
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Language-neutral error codes emitted by the gateway edge.
const (
	CodeOK                  = "OK"
	CodeNotFound            = "NOT_FOUND"
	CodeExists              = "ALREADY_EXISTS"
	CodeInvalid             = "RESOURCE_INVALID"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeTierRequired        = "TIER_REQUIRED"
	CodeKycRequired         = "KYC_REQUIRED"
	CodeRateLimited         = "RATE_LIMITED"
	CodeUpstreamUnavailable = "UPSTREAM_UNAVAILABLE"
	CodeInternal            = "INTERNAL"
)

type codeMeta struct {
	Code   string
	Status int
}

// errorsMap binds gateway biz sentinels to their wire code + HTTP status.
var errorsMap = map[error]codeMeta{
	biz.ErrResourceNotFound:     {CodeNotFound, http.StatusNotFound},
	biz.ErrResourceExists:       {CodeExists, http.StatusConflict},
	biz.ErrResourceInvalid:      {CodeInvalid, http.StatusBadRequest},
	biz.ErrResourceAccessDenied: {CodeUnauthorized, http.StatusUnauthorized},
	biz.ErrTierRequired:         {CodeTierRequired, http.StatusForbidden},
	biz.ErrKycRequired:          {CodeKycRequired, http.StatusForbidden},
	biz.ErrRateLimited:          {CodeRateLimited, http.StatusTooManyRequests},
	biz.ErrUpstreamUnavailable:  {CodeUpstreamUnavailable, http.StatusBadGateway},
}

// codeStatus maps a bare code string to its HTTP status (for HandleErrorCode).
var codeStatus = map[string]int{
	CodeOK:                  http.StatusOK,
	CodeNotFound:            http.StatusNotFound,
	CodeExists:              http.StatusConflict,
	CodeInvalid:             http.StatusBadRequest,
	CodeUnauthorized:        http.StatusUnauthorized,
	CodeTierRequired:        http.StatusForbidden,
	CodeKycRequired:         http.StatusForbidden,
	CodeRateLimited:         http.StatusTooManyRequests,
	CodeUpstreamUnavailable: http.StatusBadGateway,
	CodeInternal:            http.StatusInternalServerError,
}

// HandleError writes a language-neutral JSON error for err, mapping known biz
// sentinels to their code + status and defaulting to INTERNAL/500. err == nil
// writes an OK envelope (template parity for healthz handlers).
func HandleError(err error, w http.ResponseWriter) {
	if err == nil {
		writeCode(w, http.StatusOK, CodeOK, "ok", "")

		return
	}

	for e, meta := range errorsMap {
		if errors.Is(err, e) {
			writeCode(w, meta.Status, meta.Code, e.Error(), err.Error())

			return
		}
	}

	writeCode(w, http.StatusInternalServerError, CodeInternal, "internal server error", err.Error())
}

// HandleErrorCode writes a language-neutral error for a bare code (used by
// middlewares that have no biz error, e.g. the rate limiter).
func HandleErrorCode(code string, w http.ResponseWriter) {
	status, ok := codeStatus[code]
	if !ok {
		status = http.StatusInternalServerError
	}

	writeCode(w, status, code, code, "")
}

func writeCode(w http.ResponseWriter, status int, code, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
	})
}
