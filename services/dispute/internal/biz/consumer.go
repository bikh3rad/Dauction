package biz

import (
	"context"
	"log/slog"
)

// EventConsumer is the inbound side of the bus. dispute is publish-only — it
// emits dispute.opened / dispute.resolved and consumes nothing (escrow.released
// auto-close is intentionally out of scope; the eligibility window is left to the
// gateway/escrow, see CLAUDE.md §4). It therefore declares no subjects, and the
// eventbus runner binds no subscription. The type is kept so the runner wiring
// matches the platform's outbox+inbox shape and a consumer can be added later
// without touching the runner.
type EventConsumer struct {
	logger *slog.Logger
}

// NewEventConsumer constructs the (no-op) consumer.
func NewEventConsumer(logger *slog.Logger) *EventConsumer {
	return &EventConsumer{logger: logger.With("layer", "EventConsumer")}
}

// Subjects returns the NATS subjects this consumer subscribes to (none).
func (c *EventConsumer) Subjects() []string { return nil }

// Handle is unused while Subjects() is empty; it acks anything as a no-op so a
// future shared-stream redelivery never errors.
func (c *EventConsumer) Handle(ctx context.Context, _ []byte) error {
	c.logger.DebugContext(ctx, "dispute consumes no events; ignoring")

	return nil
}
