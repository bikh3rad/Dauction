package repo

import "github.com/google/wire"

// RepoProvider is empty: the notifier owns no domain DB and no Postgres
// repositories — it holds only ephemeral in-memory subscriptions (the Hub) and a
// best-effort open-auction Registry, both wired in internal/biz. The set is kept
// so the composition root's provider list stays uniform across services.
var RepoProvider = wire.NewSet()
