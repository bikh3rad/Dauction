package handler

import (
	"application/internal/service"

	"github.com/google/wire"
)

//	@title			Dauction API Gateway
//	@version		1.0
//	@description	Edge gateway for Dauction: authN, tier/KYC guard, rate-limiting and reverse-proxy of every /apis/* route to its backend service.

//	@contact.name	Dauction Platform
//	@contact.url	https://github.com/bikh3rad/Dauction

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

var HandlerProviderSet = wire.NewSet(
	NewServiceList,
	NewMuxHealthzHandler,
	NewProxyHandler,
)

// NewServiceList aggregates the service.Handler implementations mounted on the
// shared mux. The healthz handler owns /healthz/*; the proxy handler owns the
// /apis/* edge.
func NewServiceList(
	healthzSvc *HealthzHandler,
	proxySvc *ProxyHandler,
) []service.Handler {
	return []service.Handler{
		healthzSvc,
		proxySvc,
	}
}
