package repo

import (
	"application/internal/biz"

	"github.com/google/wire"
)

var RepoProvider = wire.NewSet(
	NewIdentityAccess,
	wire.Bind(new(biz.RepositoryAccess), new(*identityAccess)),
)
