package biz

import (
	"application/internal/entity"
	"context"

	"github.com/google/uuid"
)

// RepositoryOutbox is the persistence seam for the transactional outbox relay.
type RepositoryOutbox interface {
	FetchUnpublished(ctx context.Context, limit int) ([]entity.OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
}

// EventPublisher relays an outbox payload to the message bus (NATS/JetStream).
// Implemented by the datasource layer; the biz publisher only depends on this
// thin seam so it stays unit-testable.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}
