package entity

import (
	"time"

	"github.com/google/uuid"
)

// ObjectState is the lifecycle of a vault object (CLAUDE.md §1, §7).
// MONOSPACE_UPPERCASE protocol vocabulary; the value string IS the wire code.
//
//	IN_VAULT -> APPRAISING -> IN_AUCTION -> SOLD
//	IN_VAULT -> BOUGHT_BACK            (instant buyback, terminal)
type ObjectState string

const (
	// ObjectInVault is the resting state of an owned object.
	ObjectInVault ObjectState = "IN_VAULT"
	// ObjectAppraising means the object has been listed to auction and is
	// awaiting certification/appraisal before scheduling.
	ObjectAppraising ObjectState = "APPRAISING"
	// ObjectInAuction means the object's lot is live/scheduled in an auction.
	ObjectInAuction ObjectState = "IN_AUCTION"
	// ObjectSold is the terminal state after an auction completed with a winner.
	ObjectSold ObjectState = "SOLD"
	// ObjectBoughtBack is the terminal state after instant buyback.
	ObjectBoughtBack ObjectState = "BOUGHT_BACK"
)

// Valid reports whether s is a known object state.
func (s ObjectState) Valid() bool {
	switch s {
	case ObjectInVault, ObjectAppraising, ObjectInAuction, ObjectSold, ObjectBoughtBack:
		return true
	default:
		return false
	}
}

// VaultObject is a single item in a member's private collection. The appraised
// value drives buyback math and is int64 USDC cents (CLAUDE.md §0: money is
// int64 cents, never floats).
type VaultObject struct {
	ID                  uuid.UUID   `json:"id"`
	OwnerAccountID      uuid.UUID   `json:"ownerAccountId"`
	Title               string      `json:"title"`
	Description         string      `json:"description"`
	AppraisedValueCents int64       `json:"appraisedValueCents"`
	State               ObjectState `json:"state"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`
}

// AuctionMode is the engine an object is listed under (CLAUDE.md §1, common.proto
// AuctionMode). The value string IS the wire code.
type AuctionMode string

const (
	AuctionDutch   AuctionMode = "DUTCH"   // live descending price (active)
	AuctionVickrey AuctionMode = "VICKREY" // sealed second-price (timed, passive)
	AuctionUniqBid AuctionMode = "UNIQBID" // lowest unique price (timed, passive)
)

// Valid reports whether m is a known auction mode.
func (m AuctionMode) Valid() bool {
	switch m {
	case AuctionDutch, AuctionVickrey, AuctionUniqBid:
		return true
	default:
		return false
	}
}

// Timed reports whether m is a passive/timed mode that requires a close window
// (durationDays). DUTCH is live and has no duration.
func (m AuctionMode) Timed() bool {
	return m == AuctionVickrey || m == AuctionUniqBid
}

// ValidDurationDays reports whether days is one of the owner-selectable timed
// windows (2 / 5 / 7 days, CLAUDE.md §1).
func ValidDurationDays(days int) bool {
	return days == 2 || days == 5 || days == 7 //nolint:mnd
}

// BuybackMode is the instant-buyback payout choice (CLAUDE.md §1, common.proto
// BuybackMode). The value string IS the wire code.
type BuybackMode string

const (
	BuybackModeCash   BuybackMode = "CASH"   // 50% in USDC cash
	BuybackModeCredit BuybackMode = "CREDIT" // 85% as Vault Credit
)

// Valid reports whether m is a known buyback mode.
func (m BuybackMode) Valid() bool {
	switch m {
	case BuybackModeCash, BuybackModeCredit:
		return true
	default:
		return false
	}
}
