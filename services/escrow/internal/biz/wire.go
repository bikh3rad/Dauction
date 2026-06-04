package biz

import (
	"log/slog"

	"github.com/google/wire"
)

// ProvideEscrow is the Wire entry point for the escrow use case. It calls
// NewEscrow with no options (the production clock); the variadic EscrowOption
// param exists only for tests (WithClock), which Wire cannot auto-provide.
func ProvideEscrow(logger *slog.Logger, repo RepositoryEscrow) *escrow {
	return NewEscrow(logger, repo)
}

var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	ProvideEscrow,
	wire.Bind(new(UsecaseEscrow), new(*escrow)),
)
