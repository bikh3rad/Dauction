package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// auctionCompleted mirrors dauction.events.v1.AuctionCompleted plus the
// settlement context vault needs to flip ownership and (optionally) credit the
// seller. The contract carries auction_id/lot_id/final_state; the producer also
// stamps the originating object_id and the seller-release terms so the owning
// service can settle without a DB->DB lookup. Missing release fields => a plain
// SOLD transition with no Vault Credit.
type auctionCompleted struct {
	AuctionID     string `json:"auction_id"`
	LotID         string `json:"lot_id"`
	ObjectID      string `json:"object_id"`
	FinalState    string `json:"final_state"`
	AsVaultCredit bool   `json:"as_vault_credit"`
	Release       *money `json:"release"` // seller release amount (USDC cents); set when settled
}

// auctionStateCompleted is the dauction.common.v1.AuctionState value name that
// marks a successful sale (a winner cleared). Other final states (CANCELLED /
// ABORTED) leave the object unsold.
const auctionStateCompleted = "COMPLETED"

// EventConsumer decodes inbound EventEnvelopes from the bus and applies them to
// the vault use case (CLAUDE.md §2 consume side): auction.completed -> object
// IN_AUCTION -> SOLD, crediting the seller when the release is Vault Credit.
// Idempotency is enforced via the inbox (consumed_event), keyed by the
// envelope's idempotency_key.
type EventConsumer struct {
	logger *slog.Logger
	vault  UsecaseVault
}

// NewEventConsumer constructs the consumer.
func NewEventConsumer(logger *slog.Logger, vault UsecaseVault) *EventConsumer {
	return &EventConsumer{
		logger: logger.With("layer", "EventConsumer"),
		vault:  vault,
	}
}

// Subjects returns the NATS subjects this consumer subscribes to.
func (c *EventConsumer) Subjects() []string {
	return []string{SubjectAuctionCompleted}
}

// Handle dispatches a raw EventEnvelope. Unknown subjects are ignored (acked) so
// a shared stream doesn't redeliver events this service does not own.
func (c *EventConsumer) Handle(ctx context.Context, raw []byte) error {
	var env eventEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}

	key := env.IdempotencyKey
	if key == "" {
		key = env.EventID
	}

	switch env.Type {
	case SubjectAuctionCompleted:
		return c.onAuctionCompleted(ctx, env.Payload, key)
	default:
		c.logger.DebugContext(ctx, "ignoring unrelated subject", "type", env.Type)

		return nil
	}
}

func (c *EventConsumer) onAuctionCompleted(ctx context.Context, payload []byte, key string) error {
	var msg auctionCompleted
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode auction.completed: %w", err)
	}

	// Only a real sale settles ownership; CANCELLED/ABORTED auctions are no-ops.
	if msg.FinalState != "" && msg.FinalState != auctionStateCompleted {
		c.logger.DebugContext(ctx, "auction not COMPLETED; ignoring", "state", msg.FinalState)

		return nil
	}

	objectID, err := uuid.Parse(msg.ObjectID)
	if err != nil {
		return fmt.Errorf("%w: auction.completed object_id %q", ErrResourceInvalid, msg.ObjectID)
	}

	var releaseCents int64
	if msg.Release != nil {
		releaseCents = msg.Release.Cents
	}

	return c.vault.SettleAuctionCompleted(ctx, AuctionCompletedInput{
		ObjectID:       objectID,
		AsVaultCredit:  msg.AsVaultCredit,
		ReleaseCents:   releaseCents,
		IdempotencyKey: scopedKey(SubjectAuctionCompleted, key),
	})
}

// scopedKey namespaces an inbound idempotency key by subject so keys from
// different producers never collide in this service's inbox.
func scopedKey(subject, key string) string {
	return fmt.Sprintf("%s:%s", subject, key)
}
