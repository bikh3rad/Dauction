package handler

import (
	"application/internal/eventbus"
	"application/internal/service"

	"github.com/google/wire"
)

//	@title			Dauction Notifier Service
//	@version		1.0
//	@description	Realtime fan-out edge for Dauction. Subscribes to domain events on NATS and broadcasts server-computed view-state (live Dutch price, passive countdowns/standings, escrow state, activity toasts) to connected clients over Server-Sent Events. Read-only — the socket makes no authority decisions.

//	@contact.name	Dauction Platform
//	@contact.url	https://github.com/bikh3rad/Dauction

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

var HandlerProviderSet = wire.NewSet(
	NewServiceList,
	NewMuxHealthzHandler,
	NewLiveHandler,
)

// NewServiceList aggregates the service.Handler implementations registered on the
// mux. It also takes the eventbus *Runner so Wire instantiates it (the runner
// self-registers its lifecycle hooks — the NATS subscription and Dutch price
// ticker — on the controller in its constructor).
func NewServiceList(
	healthzSvc *HealthzHandler,
	liveSvc *liveHandler,
	_ *eventbus.Runner,
) []service.Handler {
	return []service.Handler{
		healthzSvc,
		liveSvc,
	}
}
