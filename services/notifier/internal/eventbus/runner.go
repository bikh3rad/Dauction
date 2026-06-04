// Package eventbus wires the NATS datasource to the biz projector and drives the
// two background goroutines the notifier needs: the durable event subscription
// (events → broadcast view-state) and the Dutch price ticker (re-broadcasts the
// live descending price every tick for each OPEN Dutch auction). It sits above
// biz + datasource so neither imports the other.
package eventbus

import (
	"application/app"
	"application/internal/biz"
	"application/internal/datasource"
	"context"
	"log/slog"
	"time"

	"github.com/google/wire"
)

// ProviderSet wires the runner. NewRunner is eager-evaluated by Wire, so adding it
// to the graph registers the lifecycle hooks.
var ProviderSet = wire.NewSet(
	NewRunner,
)

// defaultTickInterval is used when notifier.tickInterval is unset/invalid.
const defaultTickInterval = time.Second

// RunnerConfig is the notifier sub-tree (koanf). TickInterval bounds how often the
// Dutch price feed re-broadcasts (target <100ms latency for a drop; default 1s).
type RunnerConfig struct {
	TickInterval string `koanf:"tickInterval"`
}

// Runner owns the background goroutines: the inbound event subscription that feeds
// the projector, and the Dutch price ticker.
type Runner struct {
	logger       *slog.Logger
	nats         *datasource.Nats
	projector    *biz.Projector
	registry     *biz.Registry
	tickInterval time.Duration

	cancel  context.CancelFunc
	stopSub func()
}

// NewRunner constructs the runner, reads the tick interval, and registers
// startup/shutdown on the controller.
func NewRunner(
	logger *slog.Logger,
	config *app.KConfig,
	nats *datasource.Nats,
	projector *biz.Projector,
	registry *biz.Registry,
	controller app.Controller,
) *Runner {
	cfg := new(RunnerConfig)
	_ = config.Unmarshal("notifier", cfg)

	interval := defaultTickInterval
	if cfg.TickInterval != "" {
		if d, err := time.ParseDuration(cfg.TickInterval); err == nil && d > 0 {
			interval = d
		}
	}

	r := &Runner{
		logger:       logger.With("layer", "EventbusRunner"),
		nats:         nats,
		projector:    projector,
		registry:     registry,
		tickInterval: interval,
	}

	controller.RegisterStartup("eventbus", r.start)
	controller.RegisterShutdown("eventbus", r.stop)

	return r
}

// start binds the inbound subscription and launches the price ticker.
func (r *Runner) start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	stop, err := r.nats.Consume(runCtx, "notifier", r.projector.Subjects(), r.projector.Handle)
	if err != nil {
		cancel()

		return err
	}

	r.stopSub = stop

	go r.tickLoop(runCtx)

	r.logger.InfoContext(ctx, "notifier eventbus runner started",
		"subjects", r.projector.Subjects(), "tickInterval", r.tickInterval.String())

	return nil
}

// tickLoop re-broadcasts the live Dutch price for every OPEN Dutch auction every
// tickInterval, so descending prices and next-drop countdowns stay fresh even
// without a new event. It exits when the run context is cancelled.
func (r *Runner) tickLoop(ctx context.Context) {
	ticker := time.NewTicker(r.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, id := range r.registry.OpenDutch() {
				r.projector.DutchPriceTick(id)
			}
		}
	}
}

// stop tears down the subscription and ticker.
func (r *Runner) stop(_ context.Context) error {
	if r.stopSub != nil {
		r.stopSub()
	}

	if r.cancel != nil {
		r.cancel()
	}

	r.logger.Info("notifier eventbus runner stopped")

	return nil
}
