package repo

import (
	"application/internal/biz"
	"application/internal/datasource"
	"context"
	"log/slog"
)

// natsPublisher relays event payloads onto NATS JetStream. The subject is the
// event type (e.g. "kyc.approved").
type natsPublisher struct {
	logger *slog.Logger
	nats   *datasource.Nats
}

var _ biz.EventPublisher = (*natsPublisher)(nil)

// NewNatsPublisher builds the NATS-backed event publisher.
func NewNatsPublisher(logger *slog.Logger, nats *datasource.Nats) *natsPublisher {
	return &natsPublisher{
		logger: logger.With("layer", "NatsPublisher"),
		nats:   nats,
	}
}

// Publish sends payload to subject via JetStream (at-least-once).
func (p *natsPublisher) Publish(ctx context.Context, subject string, payload []byte) error {
	_, err := p.nats.JetStream.Publish(ctx, subject, payload)
	if err != nil {
		p.logger.WarnContext(ctx, "jetstream publish failed", "subject", subject, "error", err)

		return err
	}

	return nil
}
