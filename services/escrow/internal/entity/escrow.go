package entity

import (
	"time"

	"github.com/google/uuid"
)

// EscrowState is the funds-ledger state machine (root CLAUDE.md §4). `escrow` is
// the sole writer of escrow rows. MONOSPACE_UPPERCASE — the value string IS the
// wire code (matches dauction.common.v1.EscrowState).
//
//	             reserve(10%)     open(100%)     hammer/won
//	UNLOCKED ──▶ DEPOSIT_LOCKED ──▶ FULL_LOCKED ──▶ HELD ──▶ RELEASED   (→ seller)
//	     ▲             │                │           │
//	     └─ refund ◀───┴── unfreeze     └────────────┼─ FORFEITED (missed 24h funding)
//	                                                 └─ DISPUTED → RELEASED | REFUNDED | SPLIT
type EscrowState string

const (
	StateUnlocked      EscrowState = "UNLOCKED"
	StateDepositLocked EscrowState = "DEPOSIT_LOCKED" // 10% reservation deposit (Dutch)
	StateFullLocked    EscrowState = "FULL_LOCKED"    // 100% locked at open (Dutch)
	StateHeld          EscrowState = "HELD"           // winner funds held (hammer/won + premium)
	StateReleased      EscrowState = "RELEASED"       // released to seller
	StateRefunded      EscrowState = "REFUNDED"       // returned to buyer (loser/dispute)
	StateForfeited     EscrowState = "FORFEITED"      // winner missed the 24h funding window
	StateDisputed      EscrowState = "DISPUTED"       // dispute court; manual release suspended
)

// Valid reports whether s is a known escrow state.
func (s EscrowState) Valid() bool {
	switch s {
	case StateUnlocked, StateDepositLocked, StateFullLocked, StateHeld,
		StateReleased, StateRefunded, StateForfeited, StateDisputed:
		return true
	default:
		return false
	}
}

// Terminal reports whether s is an end state (no further transitions).
func (s EscrowState) Terminal() bool {
	switch s {
	case StateReleased, StateRefunded, StateForfeited:
		return true
	default:
		return false
	}
}

// TradeKind distinguishes the live Dutch path (reserve→full-lock→hammer) from the
// passive path (winner funds cleared price + premium straight into HELD).
type TradeKind string

const (
	KindDutch   TradeKind = "DUTCH"
	KindPassive TradeKind = "PASSIVE"
)

// Valid reports whether k is a known trade kind.
func (k TradeKind) Valid() bool {
	return k == KindDutch || k == KindPassive
}

// ReleaseMode is the seller payout choice recorded on release (root CLAUDE.md §4):
// CASH = 100% in USDC to the seller; VAULT_CREDIT = 110% recorded as a Vault-Credit
// instruction (vault consumes the release event to credit).
type ReleaseMode string

const (
	ReleaseCash        ReleaseMode = "CASH"
	ReleaseVaultCredit ReleaseMode = "VAULT_CREDIT"
)

// Valid reports whether m is a known release mode.
func (m ReleaseMode) Valid() bool {
	return m == ReleaseCash || m == ReleaseVaultCredit
}

// DisputeRuling is the dispute court's verdict applied to a HELD/DISPUTED trade
// (root CLAUDE.md §4; mirrors dauction.common.v1.DisputeRuling).
type DisputeRuling string

const (
	RulingRefundBuyer   DisputeRuling = "REFUND_BUYER"
	RulingReleaseSeller DisputeRuling = "RELEASE_SELLER"
	RulingSplit         DisputeRuling = "SPLIT"
)

// Valid reports whether r is a known dispute ruling.
func (r DisputeRuling) Valid() bool {
	switch r {
	case RulingRefundBuyer, RulingReleaseSeller, RulingSplit:
		return true
	default:
		return false
	}
}

// EscrowTrade is the per-trade head record (trade_id == auction_id). It carries
// the obligation (price + premium + fees) and the current state. The signed
// ledger rows (entity.LedgerEntry) are the source of truth for balances; this row
// is the derived state head plus immutable trade terms.
type EscrowTrade struct {
	ID                uuid.UUID   `json:"id"` // trade_id == auction_id
	LotID             uuid.UUID   `json:"lotId"`
	BuyerAccountID    uuid.UUID   `json:"buyerAccountId"`  // winner / buyer obligor
	SellerAccountID   uuid.UUID   `json:"sellerAccountId"` // release beneficiary
	Kind              TradeKind   `json:"kind"`
	State             EscrowState `json:"state"`
	PriceCents        int64       `json:"priceCents"`        // hammer / cleared price (USDC cents)
	PremiumCents      int64       `json:"premiumCents"`      // buyer's premium (USDC cents)
	FeeCents          int64       `json:"feeCents"`          // house fee (USDC cents)
	InspectorFeeCents int64       `json:"inspectorFeeCents"` // inspector attestation fee (USDC cents)
	ReleaseMode       ReleaseMode `json:"releaseMode"`       // chosen at release; empty until then
	FundingDeadline   *time.Time  `json:"fundingDeadline"`   // winner must fund by this instant (24h window)
	CreatedAt         time.Time   `json:"createdAt"`
	UpdatedAt         time.Time   `json:"updatedAt"`
}

// ObligationCents is the total the winner must fund into HELD: price + premium.
// Fees and inspector fee are carved out of the held amount at release time, not
// added on top of the buyer obligation (they net to zero in conservation).
func (t EscrowTrade) ObligationCents() int64 {
	return t.PriceCents + t.PremiumCents
}

// LedgerEntryType is the kind of a single append-only ledger row. Signed
// amount_cents convention: locks/holds/release/forfeit are POSITIVE inflows into
// the trade pot; refunds are NEGATIVE outflows back to the buyer; fee/premium/
// inspector_fee are POSITIVE carve-outs recorded at release. MONOSPACE_UPPERCASE.
type LedgerEntryType string

const (
	EntryDepositLock  LedgerEntryType = "DEPOSIT_LOCK"  // +10% reservation (Dutch)
	EntryFullLock     LedgerEntryType = "FULL_LOCK"     // +remaining to reach 100% (Dutch)
	EntryHold         LedgerEntryType = "HOLD"          // +winner obligation funded (passive / dutch post-hammer)
	EntryRelease      LedgerEntryType = "RELEASE"       // +amount released to seller
	EntryRefund       LedgerEntryType = "REFUND"        // -amount returned to a participant
	EntryForfeit      LedgerEntryType = "FORFEIT"       // +amount forfeited (winner missed funding)
	EntryFee          LedgerEntryType = "FEE"           // +house fee carve-out
	EntryPremium      LedgerEntryType = "PREMIUM"       // +buyer premium carve-out
	EntryInspectorFee LedgerEntryType = "INSPECTOR_FEE" // +inspector fee carve-out
)

// Valid reports whether e is a known entry type.
func (e LedgerEntryType) Valid() bool {
	switch e {
	case EntryDepositLock, EntryFullLock, EntryHold, EntryRelease,
		EntryRefund, EntryForfeit, EntryFee, EntryPremium, EntryInspectorFee:
		return true
	default:
		return false
	}
}

// Inflow reports whether this entry type adds gross funds INTO the trade pot
// (the buyer locking/holding money, or a forfeit converting a hold). These are
// the entries whose sum defines the conservation total once funds are locked.
func (e LedgerEntryType) Inflow() bool {
	switch e {
	case EntryDepositLock, EntryFullLock, EntryHold:
		return true
	default:
		return false
	}
}

// Disbursement reports whether this entry type accounts for funds LEAVING the
// pot — to the seller (RELEASE), back to a participant (REFUND), to the house
// (FEE/PREMIUM/INSPECTOR_FEE) or seized (FORFEIT). The sum of disbursements must
// never exceed the conservation total and must equal it once the trade settles.
func (e LedgerEntryType) Disbursement() bool {
	switch e {
	case EntryRelease, EntryRefund, EntryForfeit, EntryFee, EntryPremium, EntryInspectorFee:
		return true
	default:
		return false
	}
}

// LedgerEntry is one append-only row in escrow_ledger. NEVER updated or deleted.
// AmountCents is SIGNED USDC cents (see LedgerEntryType for sign convention). The
// derived per-(trade, participant) balance is SUM(amount_cents).
type LedgerEntry struct {
	ID                   uuid.UUID       `json:"id"`
	TradeID              uuid.UUID       `json:"tradeId"`
	ParticipantAccountID uuid.UUID       `json:"participantAccountId"`
	EntryType            LedgerEntryType `json:"entryType"`
	AmountCents          int64           `json:"amountCents"` // signed
	Ref                  string          `json:"ref"`
	CreatedAt            time.Time       `json:"createdAt"`
}

// ParticipantBalance is a derived per-participant balance for a trade
// (SUM(amount_cents) over that participant's ledger rows).
type ParticipantBalance struct {
	ParticipantAccountID uuid.UUID `json:"participantAccountId"`
	BalanceCents         int64     `json:"balanceCents"`
}

// Conservation summarises the funds-conservation accounting for a trade's
// ledger. Inflows is the gross amount the buyer locked/held into the pot;
// Disbursed is the gross amount accounted out (release + refunds + forfeit +
// fees + premium + inspector fee). Invariant (root CLAUDE.md §4): once funds are
// locked, Inflows is constant and Disbursed never exceeds it; at settlement they
// are equal. Refund magnitudes are summed as absolute values here so a refund
// counts as a disbursement, mirroring the §4 wording
// `Σ(locked+released+refunded+forfeited+fees+premium+inspector_fee)`.
type Conservation struct {
	Inflows   int64
	Disbursed int64
}

// SummariseConservation folds a trade's ledger rows into a Conservation. Refund
// rows are stored as negative amounts; their absolute value is accumulated into
// Disbursed.
func SummariseConservation(entries []LedgerEntry) Conservation {
	var c Conservation

	for _, e := range entries {
		amt := e.AmountCents
		switch {
		case e.EntryType.Inflow():
			c.Inflows += amt
		case e.EntryType == EntryRefund:
			if amt < 0 {
				amt = -amt
			}

			c.Disbursed += amt
		case e.EntryType.Disbursement():
			c.Disbursed += amt
		}
	}

	return c
}

// Balanced reports whether the pot is fully accounted out (settled): the gross
// disbursed equals the gross locked in.
func (c Conservation) Balanced() bool { return c.Inflows == c.Disbursed }
