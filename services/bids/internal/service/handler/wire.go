package handler

import (
	"application/internal/eventbus"
	"application/internal/service"

	"github.com/google/wire"
)

//	@title			Dauction Bids Service
//	@version		1.0
//	@description	Bid-credit wallet, credit packages, purchase, and idempotent debit-on-bid for Dauction.

//	@contact.name	Dauction Platform
//	@contact.url	https://github.com/bikh3rad/Dauction

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

var HandlerProviderSet = wire.NewSet(

	NewServiceList,
	NewMuxHealthzHandler,
	NewWalletHandler,
)

// NewServiceList aggregates the service.Handler implementations registered on the
// mux. It also takes the eventbus *Runner so Wire instantiates it (the runner
// self-registers its lifecycle hooks on the controller in its constructor).
func NewServiceList(
	healthzSvc *HealthzHandler,
	walletSvc *walletHandler,
	_ *eventbus.Runner,
) []service.Handler {
	return []service.Handler{
		healthzSvc,
		walletSvc,
	}
}
