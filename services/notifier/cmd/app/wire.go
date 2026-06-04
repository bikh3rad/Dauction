//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"application/app"
	"application/internal/biz"
	"application/internal/datasource"
	"application/internal/eventbus"
	"application/internal/service"
	"application/internal/service/handler"
	"context"

	"github.com/google/wire"
)

// The notifier owns no domain DB (it holds ephemeral in-memory subscriptions), so
// repo.RepoProvider is intentionally omitted — Wire rejects an unused provider set.
func wireApp(
	ctx context.Context,
) (app.Application, error) {
	panic(wire.Build(
		app.AppProviderSet,
		datasource.DataProviderSet,
		biz.BizProviderSet,
		eventbus.ProviderSet,
		service.ServerProviderSet,
		handler.HandlerProviderSet,
	))
}
