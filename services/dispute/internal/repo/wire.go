package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewDispute,
	wire.Bind(new(biz.RepositoryDispute), new(*dispute)),

	NewOutbox,
	wire.Bind(new(biz.RepositoryOutbox), new(*outbox)),
)
