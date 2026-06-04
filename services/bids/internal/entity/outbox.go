package entity

import (
	"time"

	"github.com/google/uuid"
)

// OutboxEvent is a row in the transactional outbox. The state change (a purchase
// grant or a debit) and this row are written in ONE Postgres transaction; a
// background publisher relays unpublished rows to NATS/JetStream and marks them
// published. `Subject` is the NATS subject (the event `type`, e.g. "bids.debited");
// `Payload` is the JSON EventEnvelope. `IdempotencyKey` is producer-stable so
// consumers dedup (CLAUDE.md §0, §5).
type OutboxEvent struct {
	ID             uuid.UUID
	Subject        string
	IdempotencyKey string
	Payload        []byte
	CreatedAt      time.Time
	PublishedAt    *time.Time
}
