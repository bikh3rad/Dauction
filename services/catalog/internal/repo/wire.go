package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewLot,
	wire.Bind(new(biz.RepositoryLot), new(*lot)),

	NewOutbox,
	wire.Bind(new(biz.RepositoryOutbox), new(*outbox)),
)
