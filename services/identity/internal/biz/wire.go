package biz

import (
	"github.com/google/wire"
)

var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	NewAccount,
	wire.Bind(new(UsecaseAccount), new(*account)),

	NewAuth,
	wire.Bind(new(UsecaseAuth), new(*auth)),
)
