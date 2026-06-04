package biz

import (
	"github.com/google/wire"
)

var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	NewWallClock,

	NewAuction,
	wire.Bind(new(UsecaseAuction), new(*auction)),
)
