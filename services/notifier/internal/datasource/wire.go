package datasource

import "github.com/google/wire"

// DataProviderSet wires the notifier's only datasource: NATS/JetStream. The
// notifier owns no Postgres (it holds ephemeral in-memory subscriptions only),
// so the postgres/redis/inmemory datasources are dropped.
var DataProviderSet = wire.NewSet(
	NewNats,
)
