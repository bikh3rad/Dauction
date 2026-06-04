package repo

import (
	"application/app"
	"application/internal/biz"
	"application/internal/entity"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// accessResp mirrors identity's dto.AccessResp (the internal guard read model).
type accessResp struct {
	ID        string `json:"id"`
	Tier      string `json:"tier"`
	KycStatus string `json:"kycStatus"`
	Eligible  bool   `json:"eligible"`
}

// identityAccessConfig holds the identity base URL used by the guard.
type identityAccessConfig struct {
	BaseURL        string `koanf:"baseurl"`
	TimeoutSeconds int    `koanf:"timeoutseconds"`
}

// IdentityAccess is the RepositoryAccess implementation: an HTTP client to
// identity's internal access read model. The gateway never reaches into
// identity's DB — it consumes the contract over HTTP (root CLAUDE.md §0).
type identityAccess struct {
	logger  *slog.Logger
	tracer  trace.Tracer
	client  *http.Client
	baseURL string
}

var _ biz.RepositoryAccess = (*identityAccess)(nil)

// NewIdentityAccess builds the identity access client from koanf config under
// the `access` key (APP_ACCESS_BASEURL / APP_ACCESS_TIMEOUTSECONDS).
func NewIdentityAccess(logger *slog.Logger, kcfg *app.KConfig) (*identityAccess, error) {
	cfg := &identityAccessConfig{
		BaseURL:        "http://identity:8080",
		TimeoutSeconds: 2, //nolint:mnd
	}
	if err := kcfg.Unmarshal("access", cfg); err != nil {
		return nil, err
	}

	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 2
	}

	return &identityAccess{
		logger:  logger.With("layer", "IdentityAccessRepo"),
		tracer:  otel.Tracer("IdentityAccessRepo"),
		baseURL: cfg.BaseURL,
		client: &http.Client{
			Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}, nil
}

// FetchAccess calls GET {base}/apis/internal/accounts/{id}/access.
func (r *identityAccess) FetchAccess(ctx context.Context, accountID string) (entity.Access, error) {
	ctx, span := r.tracer.Start(ctx, "FetchAccess")
	defer span.End()

	endpoint := fmt.Sprintf("%s/apis/internal/accounts/%s/access", r.baseURL, url.PathEscape(accountID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return entity.Access{}, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.ErrorContext(ctx, "identity access call failed", "error", err)

		return entity.Access{}, biz.ErrUpstreamUnavailable
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		// fallthrough to decode
	case http.StatusNotFound:
		return entity.Access{}, biz.ErrResourceNotFound
	default:
		r.logger.ErrorContext(ctx, "identity access non-200", "status", resp.StatusCode)

		return entity.Access{}, biz.ErrUpstreamUnavailable
	}

	var body accessResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		r.logger.ErrorContext(ctx, "decode access response failed", "error", err)

		return entity.Access{}, biz.ErrUpstreamUnavailable
	}

	return entity.Access{
		ID:        body.ID,
		Tier:      entity.Tier(body.Tier),
		KycStatus: entity.KycStatus(body.KycStatus),
		Eligible:  body.Eligible,
	}, nil
}
