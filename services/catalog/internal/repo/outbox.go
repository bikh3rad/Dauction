package repo

import (
	"application/internal/biz"
	"application/internal/datasource"
	"application/internal/entity"
	"context"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type outbox struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryOutbox = (*outbox)(nil)

// NewOutbox constructs the outbox repository used by the background publisher.
func NewOutbox(logger *slog.Logger, db *datasource.PostgresDB) *outbox {
	return &outbox{
		logger: logger.With("layer", "OutboxRepo"),
		tracer: otel.Tracer("OutboxRepo"),
		db:     db,
	}
}

// FetchUnpublished implements biz.RepositoryOutbox: oldest-first unpublished rows.
func (r *outbox) FetchUnpublished(ctx context.Context, limit int) ([]entity.OutboxEvent, error) {
	ctx, span := r.tracer.Start(ctx, "outbox.FetchUnpublished")
	defer span.End()

	const query = `
		SELECT id, subject, idempotency_key, payload
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []entity.OutboxEvent

	for rows.Next() {
		var e entity.OutboxEvent
		if err := rows.Scan(&e.ID, &e.Subject, &e.IdempotencyKey, &e.Payload); err != nil {
			r.logger.WarnContext(ctx, "scan outbox row failed", "error", err)

			continue
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// MarkPublished implements biz.RepositoryOutbox.
func (r *outbox) MarkPublished(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer.Start(ctx, "outbox.MarkPublished")
	defer span.End()

	const query = `UPDATE outbox SET published_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)

	return err
}
