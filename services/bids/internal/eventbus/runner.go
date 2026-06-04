// Package eventbus wires the NATS datasource to the biz outbox publisher and
// registers it on the app lifecycle (startup/shutdown). It sits above biz +
// datasource so neither imports the other. bids only publishes (no consumer):
// it is called synchronously by auction-passive (CLAUDE.md §2, §5).
package eventbus

import (
	"application/app"
	"application/internal/biz"
	"context"
	"log/slog"

	"github.com/google/wire"
)

// ProviderSet wires the runner. NewRunner is eager-evaluated by Wire, so adding
// it to the graph is enough to register the lifecycle hooks.
var ProviderSet = wire.NewSet(
	NewRunner,
)

// Runner owns the background outbox-publisher goroutine.
type Runner struct {
	logger    *slog.Logger
	publisher *biz.OutboxPublisher

	cancel context.CancelFunc
}

// NewRunner constructs the runner and registers startup/shutdown on the controller.
func NewRunner(
	logger *slog.Logger,
	publisher *biz.OutboxPublisher,
	controller app.Controller,
) *Runner {
	r := &Runner{
		logger:    logger.With("layer", "EventbusRunner"),
		publisher: publisher,
	}

	controller.RegisterStartup("eventbus", r.start)
	controller.RegisterShutdown("eventbus", r.stop)

	return r
}

// start launches the outbox publisher loop.
func (r *Runner) start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	go r.publisher.Run(runCtx)

	r.logger.InfoContext(ctx, "eventbus runner started (publisher only)")

	return nil
}

// stop tears down the publisher loop.
func (r *Runner) stop(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}

	r.logger.Info("eventbus runner stopped")

	return nil
}
