package biz

import (
	"context"
	"log/slog"
	"time"
)

const (
	outboxBatchSize    = 100
	outboxPollInterval = 2 * time.Second
)

// OutboxPublisher is the background relay: it polls the outbox table and
// publishes unpublished rows to the bus, marking each published only after the
// broker accepts it. At-least-once delivery; consumers dedup on idempotency_key.
type OutboxPublisher struct {
	logger    *slog.Logger
	repo      RepositoryOutbox
	publisher EventPublisher
	interval  time.Duration
}

// NewOutboxPublisher constructs the relay.
func NewOutboxPublisher(logger *slog.Logger, repo RepositoryOutbox, publisher EventPublisher) *OutboxPublisher {
	return &OutboxPublisher{
		logger:    logger.With("layer", "OutboxPublisher"),
		repo:      repo,
		publisher: publisher,
		interval:  outboxPollInterval,
	}
}

// Run loops until ctx is cancelled, draining the outbox each tick.
func (p *OutboxPublisher) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox publisher stopped")

			return
		case <-ticker.C:
			if err := p.DrainOnce(ctx); err != nil {
				p.logger.WarnContext(ctx, "outbox drain failed", "error", err)
			}
		}
	}
}

// DrainOnce publishes one batch of unpublished rows, marking each row published
// only after the broker accepts it. Exported for testability; Run calls it on a
// timer. Stops at the first publish error so the next pass retries in order.
func (p *OutboxPublisher) DrainOnce(ctx context.Context) error {
	events, err := p.repo.FetchUnpublished(ctx, outboxBatchSize)
	if err != nil {
		return err
	}

	for _, e := range events {
		if err := p.publisher.Publish(ctx, e.Subject, e.Payload); err != nil {
			return err
		}

		if err := p.repo.MarkPublished(ctx, e.ID); err != nil {
			return err
		}

		p.logger.DebugContext(ctx, "published event", "subject", e.Subject, "key", e.IdempotencyKey)
	}

	return nil
}
