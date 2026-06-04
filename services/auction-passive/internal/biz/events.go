package biz

import (
	"application/internal/entity"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Subject vocabulary on the bus (CLAUDE.md §2). These are the NATS subjects /
// EventEnvelope.type values this service produces and consumes.
const (
	// emitted
	SubjectBidPlaced     = "bid.placed"
	SubjectAuctionClosed = "auction.closed"
	SubjectAuctionWon    = "auction.won"
	// consumed
	SubjectLotScheduled = "lot.scheduled"
	SubjectBidsDebited  = "bids.debited"
)

const producerName = "auction-passive"

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (this service owns only its folder), so
// we marshal the contract shape directly. `payload` carries the matching arm.
type eventEnvelope struct {
	EventID        string          `json:"event_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Producer       string          `json:"producer"`
	OccurredAt     string          `json:"occurred_at"`
	Type           string          `json:"type"`
	Version        uint32          `json:"version"`
	Payload        json.RawMessage `json:"payload"`
}

// lotScheduled mirrors catalog's emitted lot.scheduled payload (the catalog
// service is the producer; we consume its actual JSON shape, not the proto's
// dutch-centric one). Carries the atype + timing + reserve so we can create a
// passive auction without reaching into catalog's DB (CLAUDE.md §2).
type lotScheduled struct {
	LotID        string `json:"lot_id"`
	ObjectID     string `json:"object_id"`
	Mode         string `json:"mode"`          // DUTCH | VICKREY | UNIQBID (atype)
	DurationDays int32  `json:"duration_days"` // 0 for DUTCH; 2/5/7 for timed
	ScheduledAt  string `json:"scheduled_at"`  // ISO-8601 UTC
	ReserveCents int64  `json:"reserve_cents"`
	Week         string `json:"week"` // ISO week, e.g. "2026-W23"
}

// bidsDebited mirrors dauction.events.v1.BidsDebited. We consume this only for
// reconciliation/observability — the synchronous bids.Debit call made before the
// bid write is authoritative (CLAUDE.md §5).
type bidsDebited struct {
	AccountID      string        `json:"account_id"`
	Amount         bidCreditJSON `json:"amount"`
	IdempotencyKey string        `json:"idempotency_key"`
	Balance        bidCreditJSON `json:"balance"`
}

// bidCreditJSON mirrors dauction.common.v1.BidCredit (int64 whole credits).
type bidCreditJSON struct {
	Credits int64 `json:"credits"`
}

// money mirrors dauction.common.v1.Money: int64 USDC cents, never a float.
type money struct {
	Cents int64 `json:"cents"`
}

// bidPlaced mirrors dauction.events.v1.BidPlaced. The amount is sealed until
// close (consumers must not surface it pre-resolution); we still emit it so the
// notifier/escrow tail has the record.
type bidPlaced struct {
	AuctionID string `json:"auction_id"`
	BidderID  string `json:"bidder_id"`
	BidID     string `json:"bid_id"`
	Amount    money  `json:"amount"`
	PlacedAt  string `json:"placed_at"` // ISO-8601 UTC; tiebreaker key
}

// auctionClosed mirrors dauction.events.v1.AuctionClosed.
type auctionClosed struct {
	AuctionID string `json:"auction_id"`
	LotID     string `json:"lot_id"`
	Mode      string `json:"mode"`
	ClosedAt  string `json:"closed_at"`
}

// auctionWon mirrors dauction.events.v1.AuctionWon. cleared_price is the Vickrey
// 2nd-price / UniqBid lowest-unique price.
type auctionWon struct {
	AuctionID    string `json:"auction_id"`
	LotID        string `json:"lot_id"`
	WinnerID     string `json:"winner_id"`
	ClearedPrice money  `json:"cleared_price"`
	Premium      money  `json:"premium"`
}

// newBidPlacedOutbox builds the outbox row + envelope for a bid.placed emission.
// idempotencyKey is the bid id so consumers dedup.
func newBidPlacedOutbox(b entity.PassiveBid) (entity.OutboxEvent, error) {
	return newOutbox(SubjectBidPlaced, "bid:"+b.ID.String(), bidPlaced{
		AuctionID: b.AuctionID.String(),
		BidderID:  b.BidderAccountID.String(),
		BidID:     b.ID.String(),
		Amount:    money{Cents: b.PriceCents},
		PlacedAt:  b.PlacedAt.UTC().Format(time.RFC3339Nano),
	})
}

// newAuctionClosedOutbox builds the outbox row + envelope for an auction.closed
// emission (OPEN -> CLOSING). idempotencyKey is producer-stable per auction close.
func newAuctionClosedOutbox(a entity.Auction, closedAt time.Time) (entity.OutboxEvent, error) {
	return newOutbox(SubjectAuctionClosed, "auction-closed:"+a.ID.String(), auctionClosed{
		AuctionID: a.ID.String(),
		LotID:     a.LotID.String(),
		Mode:      string(a.Atype),
		ClosedAt:  closedAt.UTC().Format(time.RFC3339Nano),
	})
}

// newAuctionWonOutbox builds the outbox row + envelope for an auction.won
// emission (resolution produced a winner). idempotencyKey is producer-stable per
// auction so a replayed resolution never double-emits.
func newAuctionWonOutbox(a entity.Auction, winner uuid.UUID, clearedCents int64) (entity.OutboxEvent, error) {
	return newOutbox(SubjectAuctionWon, "auction-won:"+a.ID.String(), auctionWon{
		AuctionID:    a.ID.String(),
		LotID:        a.LotID.String(),
		WinnerID:     winner.String(),
		ClearedPrice: money{Cents: clearedCents},
		Premium:      money{Cents: 0},
	})
}

// newOutbox wraps a payload in an EventEnvelope and an outbox row for `subject`.
func newOutbox(subject, idempotencyKey string, payload any) (entity.OutboxEvent, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := json.Marshal(eventEnvelope{
		EventID:        uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		Producer:       producerName,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Type:           subject,
		Version:        1,
		Payload:        body,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        subject,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}
