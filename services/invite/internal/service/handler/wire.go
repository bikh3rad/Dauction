package handler

import (
	"application/internal/repo"
	"application/internal/service"

	"github.com/google/wire"
)

//	@title			Dauction Invite Service
//	@version		1.0
//	@description	Single-use invite codes, redemption and the invite chain.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	http://www.swagger.io/support
//	@contact.email	support@swagger.io

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

var HandlerProviderSet = wire.NewSet(
	NewServiceList,
	NewMuxHealthzHandler,
	NewInvite,
)

// NewServiceList aggregates the service handlers. The OutboxPublisher is taken as
// a dependency (though not an HTTP handler) so Wire instantiates it — its
// constructor registers the background relay on the controller.
func NewServiceList(
	healthzSvc *HealthzHandler,
	inviteSvc *invite,
	_ *repo.OutboxPublisher,
) []service.Handler {
	return []service.Handler{
		healthzSvc,
		inviteSvc,
	}
}
