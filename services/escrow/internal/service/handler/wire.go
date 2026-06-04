package handler

import (
	"application/internal/eventbus"
	"application/internal/service"

	"github.com/google/wire"
)

//	@title			Dauction Escrow Service
//	@version		1.0
//	@description	The funds ledger + escrow state machine for Dauction. Escrow is the sole writer of escrow ledger rows and enforces funds conservation (root CLAUDE.md §4). All amounts are int64 USDC cents.

//	@contact.name	Dauction Platform
//	@contact.url	https://github.com/bikh3rad/Dauction

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

var HandlerProviderSet = wire.NewSet(

	NewServiceList,
	NewMuxHealthzHandler,
	NewEscrowHandler,
)

// NewServiceList aggregates the service.Handler implementations registered on the
// mux. It also takes the eventbus *Runner so Wire instantiates it (the runner
// self-registers its lifecycle hooks on the controller in its constructor).
func NewServiceList(
	healthzSvc *HealthzHandler,
	escrowSvc *escrowHandler,
	_ *eventbus.Runner,
) []service.Handler {
	return []service.Handler{
		healthzSvc,
		escrowSvc,
	}
}
