package biz

import (
	"github.com/google/wire"
)

var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	NewWallet,
	wire.Bind(new(UsecaseWallet), new(*wallet)),

	NewOutboxPublisher,
)
