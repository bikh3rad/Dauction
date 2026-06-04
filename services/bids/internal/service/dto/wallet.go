package dto

import (
	"application/internal/biz"
	"application/internal/entity"
)

// isoLayout is the language-neutral ISO-8601 UTC timestamp layout (CLAUDE.md §0.7).
const isoLayout = "2006-01-02T15:04:05Z07:00"

// WalletResp is the read-through wallet projection (GET /apis/bids/wallet). All
// amounts are integers; `balanceCredits` is WHOLE bid credits ($1 each), never
// USDC cents (CLAUDE.md §5). The client localizes copy.
type WalletResp struct {
	AccountID      string         `json:"accountId"`
	BalanceCredits int64          `json:"balanceCredits"`
	UpdatedAt      string         `json:"updatedAt"` // ISO-8601 UTC
	Purchases      []PurchaseResp `json:"purchases"`
	Debits         []DebitResp    `json:"debits"`
}

// PurchaseResp is one recorded purchase. credits and usdcCents are DISTINCT units.
type PurchaseResp struct {
	ID             string `json:"id"`
	PackageID      string `json:"packageId"`
	CreditsGranted int64  `json:"creditsGranted"`   // whole credits
	USDCCents      int64  `json:"usdcChargedCents"` // USDC cents
	CreatedAt      string `json:"createdAt"`        // ISO-8601 UTC
}

// DebitResp is one recorded debit-on-bid.
type DebitResp struct {
	ID             string `json:"id"`
	AmountCredits  int64  `json:"amountCredits"`
	IdempotencyKey string `json:"idempotencyKey"`
	AuctionID      string `json:"auctionId"`
	CreatedAt      string `json:"createdAt"` // ISO-8601 UTC
}

// PackageResp is a purchasable credit bundle (GET /apis/bids/packages).
type PackageResp struct {
	ID         string `json:"id"`
	Credits    int64  `json:"credits"`    // whole credits granted
	PriceCents int64  `json:"priceCents"` // USDC cents
	BestValue  bool   `json:"bestValue"`
}

// BuyBidsRequest is the POST /apis/bids/buy body.
type BuyBidsRequest struct {
	PackageID      string `json:"packageId" validate:"required"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// BuyBidsResp is the POST /apis/bids/buy result.
type BuyBidsResp struct {
	CreditsGranted int64 `json:"creditsGranted"`
	USDCCents      int64 `json:"usdcChargedCents"`
	Balance        int64 `json:"balanceCredits"`
}

// DebitRequest is the POST /apis/internal/bids/debit body (called by auction-passive).
type DebitRequest struct {
	AccountID      string `json:"accountId" validate:"required,uuid"`
	Amount         int64  `json:"amount" validate:"required,gt=0"`
	IdempotencyKey string `json:"idempotencyKey" validate:"required"`
	AuctionID      string `json:"auctionId" validate:"required"`
}

// DebitResultResp is the POST /apis/internal/bids/debit result (HTTP 200 on both
// fresh burn and idempotent replay — same body).
type DebitResultResp struct {
	Amount  int64 `json:"amount"`
	Balance int64 `json:"balanceCredits"`
}

// ToWalletResp maps a biz.WalletView to its API response.
func ToWalletResp(v biz.WalletView) WalletResp {
	purchases := make([]PurchaseResp, 0, len(v.Purchases))
	for _, p := range v.Purchases {
		purchases = append(purchases, ToPurchaseResp(p))
	}

	debits := make([]DebitResp, 0, len(v.Debits))
	for _, d := range v.Debits {
		debits = append(debits, ToDebitResp(d))
	}

	return WalletResp{
		AccountID:      v.Wallet.AccountID.String(),
		BalanceCredits: v.Wallet.BalanceCredits,
		UpdatedAt:      v.Wallet.UpdatedAt.UTC().Format(isoLayout),
		Purchases:      purchases,
		Debits:         debits,
	}
}

// ToPurchaseResp maps an entity.BidPurchase.
func ToPurchaseResp(p entity.BidPurchase) PurchaseResp {
	return PurchaseResp{
		ID:             p.ID.String(),
		PackageID:      p.PackageID,
		CreditsGranted: p.CreditsGranted,
		USDCCents:      p.USDCChargedCents,
		CreatedAt:      p.CreatedAt.UTC().Format(isoLayout),
	}
}

// ToDebitResp maps an entity.BidDebit.
func ToDebitResp(d entity.BidDebit) DebitResp {
	return DebitResp{
		ID:             d.ID.String(),
		AmountCredits:  d.AmountCredits,
		IdempotencyKey: d.IdempotencyKey,
		AuctionID:      d.AuctionID,
		CreatedAt:      d.CreatedAt.UTC().Format(isoLayout),
	}
}

// ToPackageResp maps an entity.BidPackage.
func ToPackageResp(p entity.BidPackage) PackageResp {
	return PackageResp{
		ID:         p.ID,
		Credits:    p.Credits,
		PriceCents: p.PriceCents,
		BestValue:  p.BestValue,
	}
}
