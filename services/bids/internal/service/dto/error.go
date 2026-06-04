package dto

import (
	"application/internal/biz"
	"encoding/json"
	"errors"
	"net/http"
)

// ErrorResponse is the language-neutral error envelope (CLAUDE.md §0.7): `code` is
// a MONOSPACE_UPPERCASE machine code the React client localizes; `details` is a
// non-localized diagnostic string.
type ErrorResponse struct {
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

type ErrorMap map[error]struct {
	Code   string
	Status int
}

// ErrorsMap maps biz sentinels to language-neutral machine codes + HTTP status.
// An insufficient-balance debit surfaces as ErrResourceInvalid; the client maps the
// RESOURCE_INVALID code (details note the cause) to its "out of credits" copy.
var ErrorsMap = ErrorMap{
	biz.ErrResourceNotFound: {
		Code:   "RESOURCE_NOT_FOUND",
		Status: http.StatusNotFound,
	},
	biz.ErrResourceExists: {
		Code:   "RESOURCE_EXISTS",
		Status: http.StatusConflict,
	},
	biz.ErrResourceInvalid: {
		Code:   "RESOURCE_INVALID",
		Status: http.StatusBadRequest,
	},
	biz.ErrResourceAccessDenied: {
		Code:   "ACCESS_DENIED",
		Status: http.StatusUnauthorized,
	},
}

// HandleError writes the language-neutral error envelope for a biz sentinel, or a
// generic INTERNAL on an unmapped error.
func HandleError(err error, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	if err == nil {
		w.WriteHeader(http.StatusOK)

		_ = encoder.Encode(ErrorResponse{Code: "OK"})

		return
	}

	for e, v := range ErrorsMap {
		if errors.Is(err, e) {
			w.WriteHeader(v.Status)
			_ = encoder.Encode(ErrorResponse{
				Code:    v.Code,
				Details: err.Error(),
			})

			return
		}
	}

	w.WriteHeader(http.StatusInternalServerError)

	_ = encoder.Encode(ErrorResponse{
		Code:    "INTERNAL",
		Details: err.Error(),
	})
}
