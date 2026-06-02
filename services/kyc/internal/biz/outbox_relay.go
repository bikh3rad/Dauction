package biz

import (
	"context"
	"log/slog"
	"time"
)

// OutboxRelay polls the outbox table and publishes unpublished rows to the bus,
// marking each published on success. It is registered as a startup component
// and stops when its context is cancelled.
type OutboxRelay struct {
	logger    *slog.Logger
	repo      RepositoryKyc
	publisher EventPublisher
	interval  time.Duration
	batch     int
}

const (
	defaultRelayInterval = 2 * time.Second
	defaultRelayBatch    = 50
)

// NewOutboxRelay builds the relay.
func NewOutboxRelay(logger *slog.Logger, repo RepositoryKyc, publisher EventPublisher) *OutboxRelay {
	return &OutboxRelay{
		logger:    logger.With("layer", "OutboxRelay"),
		repo:      repo,
		publisher: publisher,
		interval:  defaultRelayInterval,
		batch:     defaultRelayBatch,
	}
}

// Run loops until ctx is cancelled, draining the outbox each tick.
func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("outbox relay stopping")

			return
		case <-ticker.C:
			r.drain(ctx)
		}
	}
}

func (r *OutboxRelay) drain(ctx context.Context) {
	events, err := r.repo.FetchUnpublished(ctx, r.batch)
	if err != nil {
		r.logger.WarnContext(ctx, "fetch unpublished failed", "error", err)

		return
	}

	for i := range events {
		ev := events[i]
		if err := r.publisher.Publish(ctx, ev.Subject, ev.Payload); err != nil {
			r.logger.WarnContext(ctx, "publish failed", "subject", ev.Subject, "error", err)

			continue
		}

		if err := r.repo.MarkPublished(ctx, ev.ID, time.Now()); err != nil {
			r.logger.WarnContext(ctx, "mark published failed", "id", ev.ID, "error", err)
		}
	}
}
