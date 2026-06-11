package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewAccount,
	wire.Bind(new(biz.RepositoryAccount), new(*account)),
	wire.Bind(new(biz.RepositoryAuth), new(*account)),

	NewOutbox,
	wire.Bind(new(biz.RepositoryOutbox), new(*outbox)),
)
