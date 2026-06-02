package dto

import (
	"application/internal/biz"
	"encoding/json"
	"errors"
	"net/http"
)

type ErrorResponse struct {
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type ErrorMap map[error]struct {
	Message string
	Code    int
}

// ErrorsMap maps biz sentinels to language-neutral machine codes + HTTP status.
// The React client localizes these codes; responses never carry prose copy.
var ErrorsMap = ErrorMap{
	biz.ErrResourceNotFound: {
		Message: "RESOURCE_NOT_FOUND",
		Code:    http.StatusNotFound,
	},
	biz.ErrResourceExists: {
		Message: "RESOURCE_EXISTS",
		Code:    http.StatusConflict,
	},
	biz.ErrResourceInvalid: {
		Message: "RESOURCE_INVALID",
		Code:    http.StatusBadRequest,
	},

	biz.ErrResourceAccessDenied: {
		Message: "ACCESS_DENIED",
		Code:    http.StatusUnauthorized,
	},
}

func HandleError(err error, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	if err == nil {
		w.WriteHeader(http.StatusOK)

		_ = encoder.Encode(ErrorResponse{
			Message: "OK",
		})

		return
	}

	for e, v := range ErrorsMap {
		if errors.Is(err, e) {
			w.WriteHeader(v.Code)
			_ = encoder.Encode(ErrorResponse{
				Message: v.Message,
				Details: err.Error(),
			})

			return
		}
	}

	w.WriteHeader(http.StatusInternalServerError)

	_ = encoder.Encode(ErrorResponse{
		Message: "INTERNAL_ERROR",
		Details: err.Error(),
	})
}
