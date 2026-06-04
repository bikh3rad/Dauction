package biz

import (
	"github.com/google/wire"
)

var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	NewDispute,
	wire.Bind(new(UsecaseDispute), new(*dispute)),
)
