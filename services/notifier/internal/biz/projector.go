package biz

import (
	"application/internal/entity"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Projector decodes inbound EventEnvelopes and projects each into a broadcast
// Message fanned out to the relevant room(s). It is the consume side of root
// CLAUDE.md §2/§6 for the notifier: it broadcasts ONLY server-computed view-state
// (current_price, next_drop_at, closes_at, state, escrow state) and NEVER a raw
// event or a sealed passive bid price. It makes no authority decisions.
//
// The Projector also maintains the open-auction Registry (so reconnecting clients
// get a snapshot) and drives the Dutch price ticker via the open registry.
type Projector struct {
	logger   *slog.Logger
	hub      *Hub
	registry *Registry
	clock    Clock
}

// NewProjector constructs the projector.
func NewProjector(logger *slog.Logger, hub *Hub, registry *Registry, clock Clock) *Projector {
	return &Projector{
		logger:   logger.With("layer", "Projector"),
		hub:      hub,
		registry: registry,
		clock:    clock,
	}
}

// Subjects returns the NATS subjects this projector subscribes to.
func (p *Projector) Subjects() []string { return ConsumedSubjects() }

// Handle dispatches a raw EventEnvelope to the matching projection. Unknown
// subjects are ignored (acked) so the shared stream doesn't redeliver. Decode
// errors are returned so the consumer naks for redelivery.
func (p *Projector) Handle(ctx context.Context, raw []byte) error {
	var env eventEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}

	switch env.Type {
	case SubjectAuctionOpened:
		return p.onAuctionOpened(ctx, env.Payload)
	case SubjectAuctionHammer:
		return p.onAuctionHammer(ctx, env.Payload)
	case SubjectAuctionCompleted:
		return p.onAuctionCompleted(ctx, env.Payload)
	case SubjectBidPlaced:
		return p.onBidPlaced(ctx, env.Payload)
	case SubjectAuctionClosed:
		return p.onAuctionClosed(ctx, env.Payload)
	case SubjectAuctionWon:
		return p.onAuctionWon(ctx, env.Payload)
	case SubjectEscrowLocked:
		return p.onEscrowLocked(ctx, env.Payload)
	case SubjectEscrowReleased:
		return p.onEscrowReleased(ctx, env.Payload)
	case SubjectEscrowForfeited:
		return p.onEscrowParty(ctx, env.Payload, "FORFEITED")
	case SubjectEscrowRefunded:
		return p.onEscrowParty(ctx, env.Payload, "REFUNDED")
	default:
		p.logger.DebugContext(ctx, "ignoring unrelated subject", "type", env.Type)

		return nil
	}
}

// SnapshotFor builds the snapshot Message a client receives on connect to an
// auction room, or nil if the auction isn't tracked (the client just waits for
// the next live frame). Dutch → current price + next drop; passive → closes_at.
func (p *Projector) SnapshotFor(auctionID string) *entity.Message {
	a, ok := p.registry.Get(auctionID)
	if !ok {
		return nil
	}

	msg := entity.Message{
		Kind:       entity.KindSnapshot,
		AuctionID:  auctionID,
		State:      a.State,
		Mode:       a.Mode,
		ServerTime: p.clock.Now().UTC(),
	}

	switch {
	case a.Mode == entity.ModeDutch && a.Price != nil:
		price := CurrentPrice(*a.Price, p.clock.Now())
		msg.CurrentPriceCents = &price
		msg.NextDropAt = NextDropAt(*a.Price, p.clock.Now())
	case a.ClosesAt != nil:
		msg.ClosesAt = a.ClosesAt
	}

	return &msg
}

// DutchPriceTick recomputes and broadcasts the live price for one open Dutch
// auction. The ticker loop calls this for every OPEN dutch auction each interval.
// When the price has reached the floor it still broadcasts (so the client sees the
// final floor), but next_drop_at goes nil.
func (p *Projector) DutchPriceTick(auctionID string) {
	a, ok := p.registry.Get(auctionID)
	if !ok || a.Mode != entity.ModeDutch || a.Price == nil {
		return
	}

	now := p.clock.Now()
	price := CurrentPrice(*a.Price, now)

	p.hub.Broadcast(AuctionRoom(auctionID), entity.Message{
		Kind:              entity.KindDutchPrice,
		AuctionID:         auctionID,
		State:             a.State,
		Mode:              entity.ModeDutch,
		CurrentPriceCents: &price,
		NextDropAt:        NextDropAt(*a.Price, now),
		ServerTime:        now.UTC(),
	})
}

func (p *Projector) onAuctionOpened(ctx context.Context, payload []byte) error {
	var msg auctionOpened
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode auction.opened: %w", err)
	}

	openAt, _ := time.Parse(time.RFC3339Nano, msg.OpenedAt)

	params := entity.PriceParams{
		CeilingCents:     msg.Ceiling.Cents,
		FloorCents:       msg.Floor.Cents,
		DropStepCents:    msg.DropStep.Cents,
		DropIntervalSecs: int64(msg.DropIntervalSecs),
		OpenAt:           openAt.UTC(),
	}

	p.registry.Put(entity.OpenAuction{
		AuctionID: msg.AuctionID,
		Mode:      entity.ModeDutch,
		State:     "OPEN",
		Price:     &params,
	})

	now := p.clock.Now()
	price := CurrentPrice(params, now)

	p.hub.Broadcast(AuctionRoom(msg.AuctionID), entity.Message{
		Kind:              entity.KindDutchPrice,
		AuctionID:         msg.AuctionID,
		State:             "OPEN",
		Mode:              entity.ModeDutch,
		CurrentPriceCents: &price,
		NextDropAt:        NextDropAt(params, now),
		ServerTime:        now.UTC(),
	})

	p.logger.DebugContext(ctx, "dutch auction opened", "auction", msg.AuctionID)

	return nil
}

func (p *Projector) onAuctionHammer(ctx context.Context, payload []byte) error {
	var msg auctionHammer
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode auction.hammer: %w", err)
	}

	// Stop the ticker by flipping the registry state out of OPEN.
	p.registry.SetState(msg.AuctionID, "HAMMER")

	hammer := msg.HammerPrice.Cents

	p.hub.Broadcast(AuctionRoom(msg.AuctionID), entity.Message{
		Kind:         entity.KindHammer,
		AuctionID:    msg.AuctionID,
		State:        "HAMMER",
		Mode:         entity.ModeDutch,
		WinnerID:     msg.WinnerID,
		ClearedCents: &hammer,
		ServerTime:   p.clock.Now().UTC(),
	})

	p.logger.DebugContext(ctx, "dutch hammer", "auction", msg.AuctionID)

	return nil
}

func (p *Projector) onAuctionCompleted(ctx context.Context, payload []byte) error {
	var msg auctionCompleted
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode auction.completed: %w", err)
	}

	p.hub.Broadcast(AuctionRoom(msg.AuctionID), entity.Message{
		Kind:       entity.KindCompleted,
		AuctionID:  msg.AuctionID,
		State:      msg.FinalState,
		ServerTime: p.clock.Now().UTC(),
	})

	p.registry.Remove(msg.AuctionID)

	p.logger.DebugContext(ctx, "auction completed", "auction", msg.AuctionID, "state", msg.FinalState)

	return nil
}

func (p *Projector) onBidPlaced(ctx context.Context, payload []byte) error {
	var msg bidPlaced
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode bid.placed: %w", err)
	}

	// CRITICAL (root CLAUDE.md §6): a passive bid is sealed. Broadcast only an
	// activity toast — NEVER msg.Amount, the bidder, or any price.
	p.hub.Broadcast(AuctionRoom(msg.AuctionID), entity.Message{
		Kind:       entity.KindActivity,
		AuctionID:  msg.AuctionID,
		BidCount:   1,
		ServerTime: p.clock.Now().UTC(),
	})

	p.logger.DebugContext(ctx, "passive bid activity", "auction", msg.AuctionID)

	return nil
}

func (p *Projector) onAuctionClosed(ctx context.Context, payload []byte) error {
	var msg auctionClosed
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode auction.closed: %w", err)
	}

	p.registry.SetState(msg.AuctionID, "CLOSING")

	p.hub.Broadcast(AuctionRoom(msg.AuctionID), entity.Message{
		Kind:       entity.KindClosed,
		AuctionID:  msg.AuctionID,
		State:      "CLOSING",
		Mode:       msg.Mode,
		ServerTime: p.clock.Now().UTC(),
	})

	p.logger.DebugContext(ctx, "passive auction closed", "auction", msg.AuctionID)

	return nil
}

func (p *Projector) onAuctionWon(ctx context.Context, payload []byte) error {
	var msg auctionWon
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode auction.won: %w", err)
	}

	cleared := msg.ClearedPrice.Cents

	p.hub.Broadcast(AuctionRoom(msg.AuctionID), entity.Message{
		Kind:         entity.KindWon,
		AuctionID:    msg.AuctionID,
		State:        "RESOLVED",
		WinnerID:     msg.WinnerID,
		ClearedCents: &cleared,
		ServerTime:   p.clock.Now().UTC(),
	})

	p.registry.Remove(msg.AuctionID)

	p.logger.DebugContext(ctx, "passive auction won", "auction", msg.AuctionID)

	return nil
}

func (p *Projector) onEscrowLocked(ctx context.Context, payload []byte) error {
	var msg escrowLocked
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode escrow.locked: %w", err)
	}

	amt := msg.Amount.Cents

	p.broadcastEscrow(msg.TradeID, msg.ParticipantID, msg.State, &amt)

	p.logger.DebugContext(ctx, "escrow locked", "trade", msg.TradeID, "state", msg.State)

	return nil
}

func (p *Projector) onEscrowReleased(ctx context.Context, payload []byte) error {
	var msg escrowReleased
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode escrow.released: %w", err)
	}

	amt := msg.Amount.Cents

	// Released funds go to the seller; broadcast to the trade (auction) room and
	// the seller's me-room.
	p.broadcastEscrow(msg.TradeID, msg.SellerID, "RELEASED", &amt)

	p.logger.DebugContext(ctx, "escrow released", "trade", msg.TradeID)

	return nil
}

func (p *Projector) onEscrowParty(ctx context.Context, payload []byte, state string) error {
	var msg escrowParty
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode escrow.%s: %w", state, err)
	}

	amt := msg.Amount.Cents

	p.broadcastEscrow(msg.TradeID, msg.ParticipantID, state, &amt)

	p.logger.DebugContext(ctx, "escrow party state", "trade", msg.TradeID, "state", state)

	return nil
}

// broadcastEscrow fans an escrow state change to both the trade's auction room
// (the trade_id == auction_id, root §4) and the affected participant's me-room.
func (p *Projector) broadcastEscrow(tradeID, participantID, state string, amountCents *int64) {
	now := p.clock.Now().UTC()

	if participantID != "" {
		p.hub.Broadcast(MeRoom(participantID), entity.Message{
			Kind:        entity.KindEscrowState,
			AuctionID:   tradeID,
			AccountID:   participantID,
			State:       state,
			AmountCents: amountCents,
			ServerTime:  now,
		})
	}

	p.hub.Broadcast(AuctionRoom(tradeID), entity.Message{
		Kind:        entity.KindEscrowState,
		AuctionID:   tradeID,
		State:       state,
		AmountCents: amountCents,
		ServerTime:  now,
	})
}
