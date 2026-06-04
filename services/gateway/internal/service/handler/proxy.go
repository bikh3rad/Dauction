package handler

import (
	"application/app"
	"application/internal/biz"
	"application/internal/proxy"
	"application/internal/router"
	"application/internal/service"
	"application/internal/service/dto"
	"application/pkg/middlewares"
	"context"
	"log/slog"
	"net/http"
	"reflect"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// ProxyHandler is the gateway edge: it owns the `/apis/` mount, runs the edge
// middleware chain (request-id/logging → recovery → auth → rate-limit), then per
// request matches a route, applies the tier/KYC guard, injects trusted identity
// headers, and reverse-proxies to the resolved upstream service.
type ProxyHandler struct {
	logger *slog.Logger
	tracer trace.Tracer
	mux    *http.ServeMux

	table     *router.Table
	upstreams *app.UpstreamsConfig
	access    biz.UsecaseAccess
	proxy     *proxy.Proxy

	rateLimit *app.RateLimitConfig
}

var _ service.Handler = (*ProxyHandler)(nil)

// NewProxyHandler constructs the gateway proxy handler.
func NewProxyHandler(
	logger *slog.Logger,
	mux *http.ServeMux,
	upstreams *app.UpstreamsConfig,
	rateLimit *app.RateLimitConfig,
	accessUC biz.UsecaseAccess,
) *ProxyHandler {
	return &ProxyHandler{
		logger:    logger.With("layer", "ProxyHandler"),
		tracer:    otel.Tracer(reflect.TypeOf(ProxyHandler{}).String()),
		mux:       mux,
		table:     router.NewTable(),
		upstreams: upstreams,
		access:    accessUC,
		proxy:     proxy.New(logger),
		rateLimit: rateLimit,
	}
}

// RegisterHandler mounts the `/apis/` catch-all behind the edge middleware chain.
// Go 1.22 ServeMux treats "/apis/" as a subtree match, so every public route is
// funnelled through the single proxy entrypoint.
func (h *ProxyHandler) RegisterHandler(_ context.Context) error {
	recoverMW := middlewares.NewRecoveryMiddleware(
		middlewares.WithLogger[*middlewares.RecoverMiddleware](h.logger),
		middlewares.WithConsolePanic(false),
	)
	loggerMW := middlewares.NewHTTPLoggerMiddleware(
		middlewares.WithLogger[*middlewares.HTTPLoggerMiddleware](h.logger),
		middlewares.WithLevel[*middlewares.HTTPLoggerMiddleware](slog.LevelInfo),
	)
	authMW := middlewares.NewAuthMiddleware(h.logger)

	// Order (outermost → innermost): request-id/logging, panic recovery, auth
	// (strip spoofed identity headers + inject trusted X-Account-Id), then rate
	// limit (keyed by the resolved account when present, else client IP). The
	// guard + proxy run in the wrapped handler. Auth precedes the limiter so the
	// limiter can key on the authenticated account; both run after recovery so a
	// panic in either is contained.
	chain := []middlewares.Middleware{
		middlewares.SetRequestContextLogger,
		loggerMW.LoggerMiddleware,
		recoverMW.RecoverMiddleware,
		authMW.Middleware,
	}

	if h.rateLimit.Enabled {
		limiter := middlewares.NewRateLimiter(
			h.logger,
			h.rateLimit.Limit,
			time.Duration(h.rateLimit.WindowSeconds)*time.Second,
		)
		chain = append(chain, limiter.Middleware)
	}

	h.mux.HandleFunc("/apis/", middlewares.MultipleMiddleware(h.serve, chain...))

	return nil
}

// serve resolves the matched route, runs the guard, injects trusted headers and
// proxies. It is the innermost handler in the edge chain.
func (h *ProxyHandler) serve(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "serve")
	defer span.End()

	r = r.WithContext(ctx)
	logger := h.logger.With("method", r.Method, "path", r.URL.Path)

	route, ok := h.table.Match(r.Method, r.URL.Path)
	if !ok {
		logger.WarnContext(ctx, "no route matched")
		dto.HandleError(biz.ErrResourceNotFound, w)

		return
	}

	// auth middleware already stripped inbound identity headers and set a trusted
	// X-Account-Id when the bearer was present.
	accountID := r.Header.Get(middlewares.HeaderAccountID)

	result, err := h.access.Authorize(ctx, accountID, route.Req)
	if err != nil {
		logger.WarnContext(ctx, "authorization denied", "upstream", route.Upstream, "error", err)
		dto.HandleError(err, w)

		return
	}

	// Inject the trusted tier/KYC headers for authenticated callers so downstream
	// services (e.g. auction-dutch) can read them without re-fetching.
	if !route.Req.Public && accountID != "" {
		r.Header.Set(middlewares.HeaderAccountTier, string(result.Access.Tier))
		if result.Access.KycApproved() {
			r.Header.Set(middlewares.HeaderKycApproved, "true")
		} else {
			r.Header.Set(middlewares.HeaderKycApproved, "false")
		}
	}

	baseURL, err := h.upstreams.URL(route.Upstream)
	if err != nil {
		logger.ErrorContext(ctx, "unknown upstream", "upstream", route.Upstream, "error", err)
		dto.HandleError(biz.ErrResourceInvalid, w)

		return
	}

	span.SetName("proxy:" + route.Upstream)

	if err := h.proxy.ServeUpstream(w, r, baseURL); err != nil {
		logger.ErrorContext(ctx, "proxy failed", "upstream", route.Upstream, "error", err)
		dto.HandleError(biz.ErrUpstreamUnavailable, w)

		return
	}
}
