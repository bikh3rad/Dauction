package entity

import (
	"time"

	"github.com/google/uuid"
)

// BidWallet is the read-through credit balance for an account. balance_credits is
// in WHOLE bid credits (int64, $1 each) — a distinct unit from USDC cents
// (CLAUDE.md §0.4, §5). It is never recomputed in a handler; the stored balance is
// authoritative and only ever mutated atomically (grant on purchase, conditional
// debit on bid).
type BidWallet struct {
	AccountID      uuid.UUID `json:"accountId"`
	BalanceCredits int64     `json:"balanceCredits"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// BidPurchase records one credit-package purchase: the USDC charge (cents) and the
// credit grant commit atomically in one transaction (CLAUDE.md §5). USDCChargedCents
// and CreditsGranted are DISTINCT units and never share a column.
type BidPurchase struct {
	ID               uuid.UUID `json:"id"`
	AccountID        uuid.UUID `json:"accountId"`
	PackageID        string    `json:"packageId"`
	CreditsGranted   int64     `json:"creditsGranted"`   // whole bid credits
	USDCChargedCents int64     `json:"usdcChargedCents"` // USDC cents
	CreatedAt        time.Time `json:"createdAt"`
}

// BidDebit records one idempotent debit-on-bid. IdempotencyKey is UNIQUE: a replay
// with the same key returns the original debit and burns nothing (CLAUDE.md §5). A
// debit row is only written when the conditional balance UPDATE succeeded, so the
// row and the balance change commit together.
type BidDebit struct {
	ID             uuid.UUID `json:"id"`
	AccountID      uuid.UUID `json:"accountId"`
	AmountCredits  int64     `json:"amountCredits"`
	IdempotencyKey string    `json:"idempotencyKey"`
	AuctionID      string    `json:"auctionId"`
	CreatedAt      time.Time `json:"createdAt"`
}
