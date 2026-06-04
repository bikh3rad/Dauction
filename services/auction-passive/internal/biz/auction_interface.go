package biz

import (
	"application/internal/entity"
	"context"
	"time"

	"github.com/google/uuid"
)

// PlaceBidInput is the caller's request to place a passive bid.
type PlaceBidInput struct {
	AuctionID  uuid.UUID
	BidderID   uuid.UUID
	PriceCents int64
	RequestID  string // client-supplied; folded into the debit idempotency key
}

// Standing is the caller's own view of an auction (CLAUDE.md §6). Bids stay
// sealed until close, so a caller only ever sees their OWN bids:
//   - VICKREY: the single sealed bid they placed (Prices has 0 or 1 entry).
//   - UNIQBID: every distinct price they submitted, each flagged with whether it
//     is currently the lowest unique price (server-computed; house policy reveals
//     only this boolean, never other bidders' prices).
type Standing struct {
	Auction entity.Auction
	Prices  []StandingPrice
}

// StandingPrice is one of the caller's prices plus its current lowest-unique flag.
type StandingPrice struct {
	PriceCents     int64
	IsLowestUnique bool // UNIQBID only; always false for VICKREY
	PlacedAt       time.Time
}

// UsecaseAuction is the passive-auction use case consumed by handlers and the
// event consumer. It owns bid intake (debit-before-persist), the sealed standing
// view, and the deterministic close & resolution (CLAUDE.md §3, §5).
type UsecaseAuction interface {
	// Get returns public auction info (state, closes_at, participant count). It
	// never exposes other bidders' prices.
	Get(ctx context.Context, id uuid.UUID) (entity.Auction, int, error)

	// PlaceBid validates the auction is OPEN and the price is positive, debits one
	// bid credit via the bids service (sync, idempotent) BEFORE persisting, then
	// writes the immutable bid row + bid.placed outbox event in one tx. Out of
	// credits / closed / illegal price -> ErrResourceInvalid.
	PlaceBid(ctx context.Context, in PlaceBidInput) (entity.PassiveBid, error)

	// Standing returns the caller's own sealed view of an auction.
	Standing(ctx context.Context, auctionID, bidderID uuid.UUID) (Standing, error)

	// Close moves an OPEN auction to CLOSING, runs the mode's deterministic
	// resolution, and transitions to RESOLVED (winner found) or ABORTED (UniqBid
	// no-unique). Emits auction.closed then auction.won. Illegal state ->
	// ErrResourceInvalid.
	Close(ctx context.Context, auctionID uuid.UUID) (entity.Auction, error)

	// CreateFromLotScheduled creates an OPEN passive auction from a catalog
	// lot.scheduled event (VICKREY/UNIQBID only; DUTCH ignored upstream).
	// Idempotent on idempotencyKey via the inbox; a duplicate is a no-op success.
	CreateFromLotScheduled(ctx context.Context, in LotScheduledInput, idempotencyKey string) error
}

// LotScheduledInput is the auction-passive projection of a catalog lot.scheduled
// event used to seed an OPEN auction. closes_at is scheduled_at + duration.
type LotScheduledInput struct {
	LotID        uuid.UUID
	Mode         entity.AuctionMode
	ScheduledAt  time.Time
	DurationDays int32
	ReserveCents int64
}

// RepositoryAuction is the persistence seam (implemented by internal/repo, mocked
// in tests). Writes that emit an event do so atomically with an outbox row.
type RepositoryAuction interface {
	// Get returns the auction or ErrResourceNotFound.
	Get(ctx context.Context, id uuid.UUID) (entity.Auction, error)

	// ParticipantCount returns the number of DISTINCT bidders on an auction.
	ParticipantCount(ctx context.Context, auctionID uuid.UUID) (int, error)

	// BidsByAuction returns every bid of an auction ordered by placed_at (the
	// immutable resolution log).
	BidsByAuction(ctx context.Context, auctionID uuid.UUID) ([]entity.PassiveBid, error)

	// BidsByBidder returns the caller's own bids on an auction (placed_at order).
	BidsByBidder(ctx context.Context, auctionID, bidderID uuid.UUID) ([]entity.PassiveBid, error)

	// HasBid reports whether the bidder already has a bid on the auction (VICKREY
	// one-bid guard).
	HasBid(ctx context.Context, auctionID, bidderID uuid.UUID) (bool, error)

	// CreateAuctionTx inserts an OPEN auction AND marks the inbox key consumed in
	// one transaction. Returns ErrResourceExists when the inbox key was already
	// seen (duplicate event) or the lot already has an auction, so the use case
	// can no-op.
	CreateAuctionTx(ctx context.Context, a entity.Auction, inboxKey string) error

	// InsertBidTx inserts an immutable bid row and writes the bid.placed outbox
	// event in one tx, conditional on the auction still being OPEN and (for
	// VICKREY) the bidder having no existing bid. Returns ErrResourceInvalid when
	// the auction is not OPEN, and ErrResourceExists on a duplicate
	// (auction,bidder,price) or replayed debit key.
	InsertBidTx(ctx context.Context, b entity.PassiveBid, vickreyOneBid bool, outbox entity.OutboxEvent) error

	// CloseTx flips OPEN -> CLOSING and writes the auction.closed outbox event in
	// one tx, conditional on the row still being OPEN. Returns ErrResourceInvalid
	// when the conditional update affected no rows.
	CloseTx(ctx context.Context, auctionID uuid.UUID, closedOutbox entity.OutboxEvent) (entity.Auction, error)

	// ResolveTx flips CLOSING -> RESOLVED (winner) or CLOSING -> ABORTED (no
	// winner), records winner + cleared price, and writes the auction.won outbox
	// event (only when a winner exists), all in one tx conditional on the row
	// still being CLOSING. Returns ErrResourceInvalid on a 0-row update.
	ResolveTx(ctx context.Context, auctionID uuid.UUID, res Result, wonOutbox *entity.OutboxEvent) (entity.Auction, error)
}

// CreditDebitor is the synchronous seam to the bids service (CLAUDE.md §5).
// auction-passive calls Debit BEFORE persisting a bid; the call is idempotent on
// idempotencyKey so a retried bid write (same key) never double-burns a credit.
// Implemented in repo/ as an HTTP client (bids base URL from config); mocked in
// tests.
type CreditDebitor interface {
	// Debit spends `amount` bid credits from the account. Returns
	// ErrResourceInvalid (mapped to "out of credits") on insufficient balance.
	Debit(ctx context.Context, accountID uuid.UUID, amount int64, idempotencyKey string, auctionID uuid.UUID) error
}
