package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewWallet,
	wire.Bind(new(biz.RepositoryWallet), new(*wallet)),

	NewOutbox,
	wire.Bind(new(biz.RepositoryOutbox), new(*outbox)),
)
