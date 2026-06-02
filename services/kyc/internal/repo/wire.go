package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewKyc,
	wire.Bind(new(biz.RepositoryKyc), new(*kyc)),

	NewNatsPublisher,
	wire.Bind(new(biz.EventPublisher), new(*natsPublisher)),
)
