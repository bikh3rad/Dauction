package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// writeJSON encodes v with the given status code, logging encode failures.
func writeJSON(w http.ResponseWriter, status int, v any, logger *slog.Logger, ctx context.Context) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}

// atoiDefault parses s as an int, returning def on failure/empty.
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}

	return n
}
