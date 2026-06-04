//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"application/app"
	"application/internal/biz"
	"application/internal/repo"
	"application/internal/service"
	"application/internal/service/handler"
	"context"

	"github.com/google/wire"
)

// The gateway is stateless (no Postgres/NATS/Redis), so datasource.DataProviderSet
// is intentionally omitted — Wire rejects an unused provider set.
func wireApp(
	ctx context.Context,
) (app.Application, error) {
	panic(wire.Build(
		app.AppProviderSet,
		biz.BizProviderSet,
		service.ServerProviderSet,
		handler.HandlerProviderSet,
		repo.RepoProvider,
	))
}
