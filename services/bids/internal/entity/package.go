package entity

// BidPackage is a purchasable credit bundle (CLAUDE.md §5). Each credit = $1.
// Seed catalogue: PKG_100 → $80, PKG_50 → $45, PKG_20 → $20. Credits and the
// USDC price are DISTINCT units (whole credits vs. USDC cents) and never mixed.
// Packages are seeded by migration and read-only at runtime.
type BidPackage struct {
	ID         string `json:"id"`
	Credits    int64  `json:"credits"`    // whole bid credits granted
	PriceCents int64  `json:"priceCents"` // USDC cents charged
	BestValue  bool   `json:"bestValue"`
}
