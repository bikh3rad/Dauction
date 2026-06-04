package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"application/internal/entity"
)

// lockRequested is the escrow.lock_requested payload (auction-dutch -> escrow).
// This subject is not in proto's frozen oneof; it is escrow's internal lock
// contract (root §2 "auctions request locks/holds through escrow"). `state` is
// the target EscrowState name (DEPOSIT_LOCKED or FULL_LOCKED); `amount` is the
// increment to lock this step. seller_id lets escrow stamp the release
// beneficiary without a DB->DB lookup.
type lockRequested struct {
	TradeID   string `json:"trade_id"`
	AuctionID string `json:"auction_id"` // accepted as an alias for trade_id
	LotID     string `json:"lot_id"`
	BuyerID   string `json:"buyer_id"`
	SellerID  string `json:"seller_id"`
	State     string `json:"state"`
	Amount    money  `json:"amount"`
}

// auctionHammer mirrors dauction.events.v1.AuctionHammer (Dutch).
type auctionHammer struct {
	AuctionID   string `json:"auction_id"`
	LotID       string `json:"lot_id"`
	WinnerID    string `json:"winner_id"`
	HammerPrice money  `json:"hammer_price"`
	Premium     money  `json:"premium"`
}

// auctionWon mirrors dauction.events.v1.AuctionWon (passive). seller_id is a
// JSON extension the producer stamps so escrow can record the release
// beneficiary (the proto AuctionWon arm omits the seller); deviation noted.
type auctionWon struct {
	AuctionID    string `json:"auction_id"`
	LotID        string `json:"lot_id"`
	WinnerID     string `json:"winner_id"`
	SellerID     string `json:"seller_id"`
	ClearedPrice money  `json:"cleared_price"`
	Premium      money  `json:"premium"`
}

// disputeResolved mirrors dauction.events.v1.DisputeResolved.
type disputeResolved struct {
	DisputeID    string `json:"dispute_id"`
	TradeID      string `json:"trade_id"`
	Ruling       string `json:"ruling"`
	BuyerAmount  money  `json:"buyer_amount"`
	SellerAmount money  `json:"seller_amount"`
}

// EventConsumer decodes inbound EventEnvelopes from the bus and applies them to
// the escrow use case (root §2 consume side): escrow.lock_requested -> lock,
// auction.hammer -> HELD, auction.won -> create pending HELD, dispute.resolved ->
// ruling. Idempotency is enforced downstream via the inbox (consumed_event) and
// conditional state transitions.
type EventConsumer struct {
	logger *slog.Logger
	escrow UsecaseEscrow
}

// NewEventConsumer constructs the consumer.
func NewEventConsumer(logger *slog.Logger, escrow UsecaseEscrow) *EventConsumer {
	return &EventConsumer{
		logger: logger.With("layer", "EventConsumer"),
		escrow: escrow,
	}
}

// Subjects returns the NATS subjects this consumer subscribes to.
func (c *EventConsumer) Subjects() []string {
	return []string{
		SubjectEscrowLockRequested,
		SubjectAuctionHammer,
		SubjectAuctionWon,
		SubjectDisputeResolved,
	}
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
	case SubjectEscrowLockRequested:
		return c.onLockRequested(ctx, env.Payload, key)
	case SubjectAuctionHammer:
		return c.onAuctionHammer(ctx, env.Payload, key)
	case SubjectAuctionWon:
		return c.onAuctionWon(ctx, env.Payload, key)
	case SubjectDisputeResolved:
		return c.onDisputeResolved(ctx, env.Payload, key)
	default:
		c.logger.DebugContext(ctx, "ignoring unrelated subject", "type", env.Type)

		return nil
	}
}

func (c *EventConsumer) onLockRequested(ctx context.Context, payload []byte, key string) error {
	var msg lockRequested
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode escrow.lock_requested: %w", err)
	}

	tradeRaw := msg.TradeID
	if tradeRaw == "" {
		tradeRaw = msg.AuctionID
	}

	tradeID, err := uuid.Parse(tradeRaw)
	if err != nil {
		return fmt.Errorf("%w: lock_requested trade_id %q", ErrResourceInvalid, tradeRaw)
	}

	lotID, err := uuid.Parse(msg.LotID)
	if err != nil {
		return fmt.Errorf("%w: lock_requested lot_id %q", ErrResourceInvalid, msg.LotID)
	}

	buyer, err := uuid.Parse(msg.BuyerID)
	if err != nil {
		return fmt.Errorf("%w: lock_requested buyer_id %q", ErrResourceInvalid, msg.BuyerID)
	}

	seller, err := uuid.Parse(msg.SellerID)
	if err != nil {
		return fmt.Errorf("%w: lock_requested seller_id %q", ErrResourceInvalid, msg.SellerID)
	}

	return c.escrow.LockRequested(ctx, LockRequest{
		TradeID:         tradeID,
		LotID:           lotID,
		BuyerAccountID:  buyer,
		SellerAccountID: seller,
		State:           entity.EscrowState(msg.State),
		AmountCents:     msg.Amount.Cents,
		IdempotencyKey:  scopedKey(SubjectEscrowLockRequested, key),
	})
}

func (c *EventConsumer) onAuctionHammer(ctx context.Context, payload []byte, key string) error {
	var msg auctionHammer
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode auction.hammer: %w", err)
	}

	tradeID, err := uuid.Parse(msg.AuctionID)
	if err != nil {
		return fmt.Errorf("%w: auction.hammer auction_id %q", ErrResourceInvalid, msg.AuctionID)
	}

	winner, err := uuid.Parse(msg.WinnerID)
	if err != nil {
		return fmt.Errorf("%w: auction.hammer winner_id %q", ErrResourceInvalid, msg.WinnerID)
	}

	return c.escrow.Hammer(ctx, HammerInput{
		TradeID:          tradeID,
		WinnerID:         winner,
		HammerPriceCents: msg.HammerPrice.Cents,
		PremiumCents:     msg.Premium.Cents,
		IdempotencyKey:   scopedKey(SubjectAuctionHammer, key),
	})
}

func (c *EventConsumer) onAuctionWon(ctx context.Context, payload []byte, key string) error {
	var msg auctionWon
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode auction.won: %w", err)
	}

	tradeID, err := uuid.Parse(msg.AuctionID)
	if err != nil {
		return fmt.Errorf("%w: auction.won auction_id %q", ErrResourceInvalid, msg.AuctionID)
	}

	lotID, err := uuid.Parse(msg.LotID)
	if err != nil {
		return fmt.Errorf("%w: auction.won lot_id %q", ErrResourceInvalid, msg.LotID)
	}

	winner, err := uuid.Parse(msg.WinnerID)
	if err != nil {
		return fmt.Errorf("%w: auction.won winner_id %q", ErrResourceInvalid, msg.WinnerID)
	}

	seller, err := uuid.Parse(msg.SellerID)
	if err != nil {
		return fmt.Errorf("%w: auction.won seller_id %q", ErrResourceInvalid, msg.SellerID)
	}

	return c.escrow.Won(ctx, WonInput{
		TradeID:           tradeID,
		LotID:             lotID,
		WinnerID:          winner,
		SellerAccountID:   seller,
		ClearedPriceCents: msg.ClearedPrice.Cents,
		PremiumCents:      msg.Premium.Cents,
		IdempotencyKey:    scopedKey(SubjectAuctionWon, key),
	})
}

func (c *EventConsumer) onDisputeResolved(ctx context.Context, payload []byte, key string) error {
	var msg disputeResolved
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode dispute.resolved: %w", err)
	}

	tradeID, err := uuid.Parse(msg.TradeID)
	if err != nil {
		return fmt.Errorf("%w: dispute.resolved trade_id %q", ErrResourceInvalid, msg.TradeID)
	}

	return c.escrow.DisputeResolved(ctx, DisputeInput{
		TradeID:        tradeID,
		Ruling:         entity.DisputeRuling(msg.Ruling),
		BuyerCents:     msg.BuyerAmount.Cents,
		SellerCents:    msg.SellerAmount.Cents,
		IdempotencyKey: scopedKey(SubjectDisputeResolved, key),
	})
}

// scopedKey namespaces an inbound idempotency key by subject so keys from
// different producers never collide in this service's inbox.
func scopedKey(subject, key string) string {
	return fmt.Sprintf("%s:%s", subject, key)
}
