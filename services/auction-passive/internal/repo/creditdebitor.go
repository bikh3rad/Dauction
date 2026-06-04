package repo

import (
	"application/app"
	"application/internal/biz"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// creditDebitor is the synchronous HTTP client to the bids service (CLAUDE.md
// §5). It calls POST {baseURL}/apis/internal/bids/debit with the deterministic
// idempotency key BEFORE auction-passive persists a bid; a retried bid (same key)
// replays the debit as a no-op at bids, so a credit is never double-burned.
type creditDebitor struct {
	logger  *slog.Logger
	tracer  trace.Tracer
	client  *http.Client
	baseURL string
}

var _ biz.CreditDebitor = (*creditDebitor)(nil)

// BidsConfig is the `bids` koanf sub-tree.
type BidsConfig struct {
	BaseURL string `koanf:"baseUrl"`
}

const (
	defaultBidsBaseURL = "http://bids:8080"
	debitTimeout       = 5 * time.Second
)

// NewCreditDebitor constructs the bids HTTP client, reading the base URL from
// config (`bids.baseUrl`, default http://bids:8080).
func NewCreditDebitor(logger *slog.Logger, config *app.KConfig) *creditDebitor {
	cfg := new(BidsConfig)
	if err := config.Unmarshal("bids", cfg); err != nil {
		logger.Error("failed to unmarshal bids config; using default base URL", "error", err)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBidsBaseURL
	}

	return &creditDebitor{
		logger:  logger.With("layer", "CreditDebitor"),
		tracer:  otel.Tracer("CreditDebitor"),
		client:  &http.Client{Timeout: debitTimeout},
		baseURL: baseURL,
	}
}

// debitRequest is the wire body for the bids internal debit endpoint.
type debitRequest struct {
	AccountID      string `json:"accountId"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotencyKey"`
	AuctionID      string `json:"auctionId"`
}

// Debit implements biz.CreditDebitor. A 2xx is success (idempotent replay
// included). A 4xx (insufficient credits / invalid) maps to ErrResourceInvalid so
// the handler returns "out of credits"; a 5xx / transport error is surfaced raw
// (the bid is not persisted, the caller may retry with the same key).
func (d *creditDebitor) Debit(ctx context.Context, accountID uuid.UUID, amount int64, idempotencyKey string, auctionID uuid.UUID) error {
	ctx, span := d.tracer.Start(ctx, "creditDebitor.Debit")
	defer span.End()

	body, err := json.Marshal(debitRequest{
		AccountID:      accountID.String(),
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		AuctionID:      auctionID.String(),
	})
	if err != nil {
		return err
	}

	url := d.baseURL + "/apis/internal/bids/debit"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := d.client.Do(req)
	if err != nil {
		d.logger.WarnContext(ctx, "bids debit transport error", "error", err)

		return fmt.Errorf("bids debit call: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusConflict:
		// Idempotent replay of an already-applied debit — treat as success.
		return nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Insufficient credits / invalid request -> "out of credits".
		d.logger.InfoContext(ctx, "bids debit rejected", "status", resp.StatusCode)

		return fmt.Errorf("%w: out of credits", biz.ErrResourceInvalid)
	default:
		return fmt.Errorf("bids debit failed with status %d", resp.StatusCode)
	}
}
