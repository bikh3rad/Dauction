// Package eventbus wires the NATS datasource to the biz outbox publisher and
// event consumer, and registers them on the app lifecycle (startup/shutdown).
// It sits above biz + datasource so neither imports the other.
package eventbus

import (
	"application/app"
	"application/internal/biz"
	"application/internal/datasource"
	"context"
	"log/slog"

	"github.com/google/wire"
)

// ProviderSet wires the runner. NewRunner is eager-evaluated by Wire, so adding
// it to the graph is enough to register the lifecycle hooks.
var ProviderSet = wire.NewSet(
	biz.NewOutboxPublisher,
	biz.NewEventConsumer,
	NewRunner,
)

// Runner owns the background goroutines: the outbox publisher loop and the NATS
// subscription that feeds the event consumer.
type Runner struct {
	logger    *slog.Logger
	nats      *datasource.Nats
	publisher *biz.OutboxPublisher
	consumer  *biz.EventConsumer

	cancel  context.CancelFunc
	stopSub func()
}

// NewRunner constructs the runner and registers startup/shutdown on the controller.
func NewRunner(
	logger *slog.Logger,
	nats *datasource.Nats,
	publisher *biz.OutboxPublisher,
	consumer *biz.EventConsumer,
	controller app.Controller,
) *Runner {
	r := &Runner{
		logger:    logger.With("layer", "EventbusRunner"),
		nats:      nats,
		publisher: publisher,
		consumer:  consumer,
	}

	controller.RegisterStartup("eventbus", r.start)
	controller.RegisterShutdown("eventbus", r.stop)

	return r
}

// start launches the outbox publisher loop and binds the inbound subscription.
func (r *Runner) start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	go r.publisher.Run(runCtx)

	stop, err := r.nats.Consume(runCtx, "escrow", r.consumer.Subjects(), r.consumer.Handle)
	if err != nil {
		cancel()

		return err
	}

	r.stopSub = stop
	r.logger.InfoContext(ctx, "eventbus runner started", "subjects", r.consumer.Subjects())

	return nil
}

// stop tears down the subscription and publisher loop.
func (r *Runner) stop(_ context.Context) error {
	if r.stopSub != nil {
		r.stopSub()
	}

	if r.cancel != nil {
		r.cancel()
	}

	r.logger.Info("eventbus runner stopped")

	return nil
}
