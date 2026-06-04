// Package entity holds the notifier's plain view-state structs. The notifier owns
// no domain DB; these are the language-neutral broadcast payloads (enum state
// codes, integer USDC cents, ISO-8601 UTC timestamps) the React client localizes.
package entity

import "time"

// Broadcast message kinds. The `kind` field tells the client how to interpret a
// frame; the payload fields are server-COMPUTED view-state only (root CLAUDE.md
// §6) — never a raw domain event, and never a sealed bid price.
const (
	KindSnapshot    = "SNAPSHOT"     // sent once on connect: current room state
	KindDutchPrice  = "DUTCH_PRICE"  // live descending price tick
	KindHammer      = "HAMMER"       // Dutch hammer fell
	KindCompleted   = "COMPLETED"    // auction completed/cancelled/aborted
	KindClosed      = "CLOSED"       // passive auction closed (resolution pending)
	KindWon         = "WON"          // passive auction resolved with a winner
	KindActivity    = "ACTIVITY"     // a passive bid landed (count/toast only)
	KindEscrowState = "ESCROW_STATE" // escrow trade state change (me-room)
	KindCountdown   = "COUNTDOWN"    // passive closes_at countdown
)

// AuctionMode mirrors the auction atype on the wire.
const (
	ModeDutch   = "DUTCH"
	ModeVickrey = "VICKREY"
	ModeUniqBid = "UNIQBID"
)

// Message is one broadcast frame fanned out to a room. It is the SSE `data:`
// payload. Only the fields relevant to `Kind` are populated; the rest are
// omitempty so the frame stays compact and language-neutral.
type Message struct {
	Kind      string `json:"kind"`
	AuctionID string `json:"auctionId,omitempty"`
	AccountID string `json:"accountId,omitempty"`

	// Dutch live price (KindDutchPrice / KindSnapshot for an open Dutch auction).
	CurrentPriceCents *int64     `json:"currentPriceCents,omitempty"`
	NextDropAt        *time.Time `json:"nextDropAt,omitempty"`

	// State is the MONOSPACE_UPPERCASE auction/escrow state name (e.g. OPEN,
	// HAMMER, COMPLETED, CLOSING, RESOLVED, HELD, RELEASED).
	State string `json:"state,omitempty"`
	Mode  string `json:"mode,omitempty"`

	// Resolution outcome (KindHammer / KindWon).
	WinnerID     string `json:"winnerId,omitempty"`
	ClearedCents *int64 `json:"clearedPriceCents,omitempty"`

	// Passive countdown (KindCountdown / KindSnapshot for an open passive auction).
	ClosesAt *time.Time `json:"closesAt,omitempty"`

	// Activity toast (KindActivity): a coarse participant/bid count — NEVER the
	// sealed price.
	BidCount int `json:"bidCount,omitempty"`

	// Escrow (KindEscrowState): the amount moved, in USDC cents.
	AmountCents *int64 `json:"amountCents,omitempty"`

	// ServerTime stamps every frame so the client can reconcile clock skew when
	// rendering countdowns. ISO-8601 UTC.
	ServerTime time.Time `json:"serverTime"`
}

// PriceParams are the immutable Dutch price parameters the notifier carries for an
// open auction so it can re-compute current_price(now) itself (root CLAUDE.md §3),
// identically to auction-dutch. All cents are int64 USDC; interval is seconds.
type PriceParams struct {
	CeilingCents     int64
	FloorCents       int64
	DropStepCents    int64
	DropIntervalSecs int64
	OpenAt           time.Time
}

// OpenAuction is the in-memory registry record for a currently-open auction, so a
// reconnecting client gets an accurate snapshot on connect. Dutch auctions carry
// price params + the live ticker; passive auctions carry closes_at.
type OpenAuction struct {
	AuctionID string
	Mode      string // DUTCH | VICKREY | UNIQBID
	State     string // OPEN | CLOSING | ...

	// Dutch only.
	Price *PriceParams

	// Passive only.
	ClosesAt *time.Time
}
