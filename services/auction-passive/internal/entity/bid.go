package entity

import (
	"time"

	"github.com/google/uuid"
)

// PassiveBid is one row in the IMMUTABLE, append-only bid log (CLAUDE.md §3, §7).
// It is never updated or deleted; resolution is a pure function of this log plus
// PlacedAt (the tiebreaker clock). PriceCents is int64 USDC cents. Every accepted
// bid carries a confirmed bids.Debit, recorded by DebitIdempotencyKey so a credit
// is never burned without a recorded bid.
//
//   - VICKREY enforces one bid per (auction, bidder) (UNIQUE).
//   - UNIQBID lets a bidder submit many DISTINCT prices (UNIQUE (auction, bidder, price)).
type PassiveBid struct {
	ID                  uuid.UUID
	AuctionID           uuid.UUID
	BidderAccountID     uuid.UUID
	PriceCents          int64
	PlacedAt            time.Time // server clock; resolution tiebreaker
	DebitIdempotencyKey string
	CreatedAt           time.Time
}
