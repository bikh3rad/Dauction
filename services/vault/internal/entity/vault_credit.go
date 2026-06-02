package entity

import (
	"time"

	"github.com/google/uuid"
)

// CreditReason is the machine reason code for a vault-credit ledger entry
// (CLAUDE.md §7). MONOSPACE_UPPERCASE; the value string IS the wire code.
type CreditReason string

const (
	// CreditBuyback credits 85% of appraised value when an owner takes the
	// Vault-Credit buyback (CLAUDE.md §1).
	CreditBuyback CreditReason = "BUYBACK"
	// CreditAuctionRelease credits the seller's release proceeds when the
	// escrow release is paid as Vault Credit (110%, CLAUDE.md §4).
	CreditAuctionRelease CreditReason = "AUCTION_RELEASE"
	// CreditAdjustment is a house/manual correction.
	CreditAdjustment CreditReason = "ADJUSTMENT"
)

// Valid reports whether r is a known credit reason.
func (r CreditReason) Valid() bool {
	switch r {
	case CreditBuyback, CreditAuctionRelease, CreditAdjustment:
		return true
	default:
		return false
	}
}

// VaultCreditEntry is one append-only row in the vault-credit ledger. The
// balance for an account is SUM(delta_cents). DeltaCents is signed int64 USDC
// cents of Vault Credit — this is USDC-denominated, NOT bid credits
// (CLAUDE.md §0, §5: never mix the two units).
type VaultCreditEntry struct {
	ID         uuid.UUID    `json:"id"`
	AccountID  uuid.UUID    `json:"accountId"`
	DeltaCents int64        `json:"deltaCents"`
	Reason     CreditReason `json:"reason"`
	RefID      string       `json:"refId"`
	CreatedAt  time.Time    `json:"createdAt"`
}
