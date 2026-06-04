package proxy

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"application/internal/service/dto"
)

// Proxy is a per-upstream reverse-proxy pool. It lazily builds (and caches) a
// httputil.ReverseProxy per upstream base URL. httputil.ReverseProxy transparently
// supports HTTP/1.1 Connection: Upgrade (so notifier WebSocket / SSE streams pass
// through unmodified) and streams bodies without buffering.
type Proxy struct {
	logger *slog.Logger

	mu      sync.RWMutex
	proxies map[string]*httputil.ReverseProxy
}

// New constructs an empty proxy pool.
func New(logger *slog.Logger) *Proxy {
	return &Proxy{
		logger:  logger.With("layer", "Proxy"),
		proxies: make(map[string]*httputil.ReverseProxy),
	}
}

// ServeUpstream proxies r/w to the given upstream base URL. Returns an error only
// when the base URL is unparseable; transport failures are surfaced to the client
// as a 502 via the proxy's ErrorHandler.
func (p *Proxy) ServeUpstream(w http.ResponseWriter, r *http.Request, baseURL string) error {
	rp, err := p.forBase(baseURL)
	if err != nil {
		return err
	}

	rp.ServeHTTP(w, r)

	return nil
}

func (p *Proxy) forBase(baseURL string) (*httputil.ReverseProxy, error) {
	p.mu.RLock()
	rp, ok := p.proxies[baseURL]
	p.mu.RUnlock()

	if ok {
		return rp, nil
	}

	target, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	rp = httputil.NewSingleHostReverseProxy(target)

	// Preserve the upstream host and tag the proxied request so upstream logs/
	// tracing can attribute it to the gateway.
	base := rp.Director
	rp.Director = func(req *http.Request) {
		base(req)
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	}

	rp.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
		p.logger.ErrorContext(req.Context(), "upstream proxy error", "target", baseURL, "error", e)
		dto.HandleErrorCode(dto.CodeUpstreamUnavailable, rw)
	}

	p.mu.Lock()
	p.proxies[baseURL] = rp
	p.mu.Unlock()

	return rp, nil
}

// Warm is a no-op placeholder so callers can pre-touch a base if desired; kept
// for symmetry and to satisfy context-aware initialization patterns.
func (p *Proxy) Warm(_ context.Context, baseURL string) error {
	_, err := p.forBase(baseURL)

	return err
}
