package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewInvite,
	wire.Bind(new(biz.RepositoryInvite), new(*invite)),

	// Background outbox -> NATS relay (registers its own startup/shutdown).
	NewOutboxPublisher,
)
