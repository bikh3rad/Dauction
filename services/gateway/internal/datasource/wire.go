package datasource

import "github.com/google/wire"

// DataProviderSet is empty: the gateway is stateless — it owns no Postgres, NATS
// or Redis. The set is kept so the cmd/app wire build references a stable symbol.
var DataProviderSet = wire.NewSet()
