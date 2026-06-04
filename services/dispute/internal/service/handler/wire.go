package handler

import (
	"application/internal/eventbus"
	"application/internal/service"

	"github.com/google/wire"
)

//	@title			Dauction Dispute Service
//	@version		1.0
//	@description	Dispute court for Dauction: buyers raise post-delivery claims, the house rules, escrow executes the verdict.

//	@contact.name	Dauction Platform
//	@contact.url	https://github.com/bikh3rad/Dauction

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

var HandlerProviderSet = wire.NewSet(

	NewServiceList,
	NewMuxHealthzHandler,
	NewDisputeHandler,
)

// NewServiceList aggregates the service.Handler implementations registered on the
// mux. It also takes the eventbus *Runner so Wire instantiates it (the runner
// self-registers its lifecycle hooks on the controller in its constructor).
func NewServiceList(
	healthzSvc *HealthzHandler,
	disputeSvc *disputeHandler,
	_ *eventbus.Runner,
) []service.Handler {
	return []service.Handler{
		healthzSvc,
		disputeSvc,
	}
}
