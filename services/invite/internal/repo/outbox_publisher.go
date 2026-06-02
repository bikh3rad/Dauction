package repo

import (
	"application/app"
	"application/internal/datasource"
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// OutboxPublisher is the background relay that drains the outbox table to NATS
// JetStream. The outbox row is written in the same tx as the domain change
// (see invite.Redeem), so the relay guarantees at-least-once delivery while the
// EventEnvelope.idempotency_key lets consumers dedup.
type OutboxPublisher struct {
	logger   *slog.Logger
	db       *datasource.PostgresDB
	nats     *datasource.Nats
	interval time.Duration
	batch    int

	cancel context.CancelFunc
	done   chan struct{}
}

// outboxEnvelope mirrors proto dauction.events.v1.EventEnvelope as published JSON.
type outboxEnvelope struct {
	EventID        string          `json:"event_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Producer       string          `json:"producer"`
	OccurredAt     string          `json:"occurred_at"`
	Type           string          `json:"type"`
	Version        uint32          `json:"version"`
	Payload        json.RawMessage `json:"payload"`
}

// NewOutboxPublisher constructs the relay and registers its lifecycle hooks on
// the shared controller (startup launches the poll loop, shutdown stops it).
func NewOutboxPublisher(
	logger *slog.Logger,
	db *datasource.PostgresDB,
	nats *datasource.Nats,
	controller app.Controller,
) *OutboxPublisher {
	p := &OutboxPublisher{
		logger:   logger.With("layer", "OutboxPublisher"),
		db:       db,
		nats:     nats,
		interval: 2 * time.Second,
		batch:    100,
		done:     make(chan struct{}),
	}

	controller.RegisterStartup("outbox-publisher", p.start)
	controller.RegisterShutdown("outbox-publisher", p.shutdown)

	return p
}

func (p *OutboxPublisher) start(ctx context.Context) error {
	loopCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	go p.loop(loopCtx)

	p.logger.InfoContext(ctx, "outbox publisher started", "interval", p.interval)

	return nil
}

func (p *OutboxPublisher) shutdown(ctx context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}

	select {
	case <-p.done:
	case <-ctx.Done():
	}

	return nil
}

func (p *OutboxPublisher) loop(ctx context.Context) {
	defer close(p.done)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.drain(ctx); err != nil {
				p.logger.WarnContext(ctx, "outbox drain failed", "error", err)
			}
		}
	}
}

// drain publishes undelivered outbox rows, marking each delivered on success.
func (p *OutboxPublisher) drain(ctx context.Context) error {
	query := `
		SELECT id, event_id, idempotency_key, producer, type, version, occurred_at, payload
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY occurred_at ASC
		LIMIT $1`

	rows, err := p.db.QueryContext(ctx, query, p.batch)
	if err != nil {
		return err
	}

	type pending struct {
		id  uuid.UUID
		env outboxEnvelope
	}

	var pendings []pending
	for rows.Next() {
		var (
			id      uuid.UUID
			env     outboxEnvelope
			payload string
		)
		if err := rows.Scan(&id, &env.EventID, &env.IdempotencyKey, &env.Producer,
			&env.Type, &env.Version, &env.OccurredAt, &payload); err != nil {
			p.logger.WarnContext(ctx, "failed to scan outbox row", "error", err)

			continue
		}
		env.Payload = json.RawMessage(payload)
		pendings = append(pendings, pending{id: id, env: env})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, pn := range pendings {
		body, err := json.Marshal(pn.env)
		if err != nil {
			p.logger.WarnContext(ctx, "failed to marshal envelope", "error", err)

			continue
		}

		// Subject is the event type, e.g. "invite.redeemed".
		if _, err := p.nats.JetStream.Publish(ctx, pn.env.Type, body); err != nil {
			p.logger.WarnContext(ctx, "failed to publish event", "type", pn.env.Type, "error", err)

			continue // leave unpublished; retried next tick
		}

		if _, err := p.db.ExecContext(ctx,
			`UPDATE outbox SET published_at = NOW() WHERE id = $1`, pn.id); err != nil {
			p.logger.WarnContext(ctx, "failed to mark outbox published", "id", pn.id, "error", err)
		}
	}

	return nil
}
