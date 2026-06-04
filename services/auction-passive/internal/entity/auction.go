package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuctionMode picks which passive engine resolves a lot (CLAUDE.md §1, §3). This
// service runs only the two timed/passive modes; DUTCH lives in auction-dutch.
// MONOSPACE_UPPERCASE protocol vocabulary — the value string IS the wire code.
type AuctionMode string

const (
	ModeVickrey AuctionMode = "VICKREY" // sealed second-price
	ModeUniqBid AuctionMode = "UNIQBID" // lowest unique price
)

// Valid reports whether m is a passive auction mode this service owns.
func (m AuctionMode) Valid() bool {
	switch m {
	case ModeVickrey, ModeUniqBid:
		return true
	default:
		return false
	}
}

// AuctionState is the passive auction lifecycle (CLAUDE.md §3). The timed window
// runs in OPEN until closes_at; CLOSING runs the deterministic resolution; a
// UniqBid with no unique price terminates in ABORTED. SETTLING/COMPLETED are the
// escrow → delivery → release tail (driven by other services' events).
type AuctionState string

const (
	StateDraft      AuctionState = "DRAFT"
	StateAppraising AuctionState = "APPRAISING"
	StateScheduled  AuctionState = "SCHEDULED"
	StateOpen       AuctionState = "OPEN"
	StateClosing    AuctionState = "CLOSING"
	StateResolved   AuctionState = "RESOLVED"
	StateSettling   AuctionState = "SETTLING"
	StateCompleted  AuctionState = "COMPLETED"
	StateAborted    AuctionState = "ABORTED"
)

// Valid reports whether s is a known auction state.
func (s AuctionState) Valid() bool {
	switch s {
	case StateDraft, StateAppraising, StateScheduled, StateOpen,
		StateClosing, StateResolved, StateSettling, StateCompleted, StateAborted:
		return true
	default:
		return false
	}
}

// AcceptsBids reports whether the auction may take new bids (OPEN only).
func (s AuctionState) AcceptsBids() bool { return s == StateOpen }

// CanClose reports whether the auction may transition OPEN -> CLOSING.
func (s AuctionState) CanClose() bool { return s == StateOpen }

// Auction is the timed passive lot. Money fields are int64 USDC cents (CLAUDE.md
// §0). WinnerAccountID / ClearedPriceCents are set at resolution (CLOSING ->
// RESOLVED); they stay nil/0 until then and for an ABORTED UniqBid.
type Auction struct {
	ID                uuid.UUID
	LotID             uuid.UUID
	Atype             AuctionMode
	State             AuctionState
	ClosesAt          time.Time
	ReserveCents      int64
	WinnerAccountID   *uuid.UUID
	ClearedPriceCents int64
	CreatedAt         time.Time
}
