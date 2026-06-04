package biz

import (
	"github.com/google/wire"
)

var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	NewAuction,
	wire.Bind(new(UsecaseAuction), new(*auction)),
)
