package datasource

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var DataProviderSet = wire.NewSet(
	NewPostgresDB,

	NewNats,
	wire.Bind(new(biz.EventPublisher), new(*Nats)),
)
