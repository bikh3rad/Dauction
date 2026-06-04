package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewEscrow,
	wire.Bind(new(biz.RepositoryEscrow), new(*escrowRepo)),

	NewOutbox,
	wire.Bind(new(biz.RepositoryOutbox), new(*outbox)),
)
