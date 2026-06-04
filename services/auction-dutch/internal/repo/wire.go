package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewAuction,
	wire.Bind(new(biz.RepositoryAuction), new(*auction)),

	NewOutbox,
	wire.Bind(new(biz.RepositoryOutbox), new(*outbox)),
)
