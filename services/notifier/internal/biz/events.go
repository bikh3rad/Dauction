package biz

import "encoding/json"

// Subject vocabulary on the bus (root CLAUDE.md §2). These are the NATS subjects /
// EventEnvelope.type values the notifier CONSUMES. The notifier emits nothing.
const (
	// dutch
	SubjectAuctionOpened    = "auction.opened"
	SubjectAuctionHammer    = "auction.hammer"
	SubjectAuctionCompleted = "auction.completed"
	// passive
	SubjectBidPlaced     = "bid.placed"
	SubjectAuctionClosed = "auction.closed"
	SubjectAuctionWon    = "auction.won"
	// escrow
	SubjectEscrowLocked    = "escrow.locked"
	SubjectEscrowReleased  = "escrow.released"
	SubjectEscrowForfeited = "escrow.forfeited"
	SubjectEscrowRefunded  = "escrow.refunded"
)

// ConsumedSubjects is the full subscription filter list (durable `notifier`).
func ConsumedSubjects() []string {
	return []string{
		SubjectAuctionOpened,
		SubjectAuctionHammer,
		SubjectAuctionCompleted,
		SubjectBidPlaced,
		SubjectAuctionClosed,
		SubjectAuctionWon,
		SubjectEscrowLocked,
		SubjectEscrowReleased,
		SubjectEscrowForfeited,
		SubjectEscrowRefunded,
	}
}

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (the notifier owns only its folder), so
// we unmarshal the contract shape directly. `payload` carries the matching arm.
type eventEnvelope struct {
	EventID        string          `json:"event_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Producer       string          `json:"producer"`
	OccurredAt     string          `json:"occurred_at"`
	Type           string          `json:"type"`
	Version        uint32          `json:"version"`
	Payload        json.RawMessage `json:"payload"`
}

// money mirrors dauction.common.v1.Money: int64 USDC cents, never a float.
type money struct {
	Cents int64 `json:"cents"`
}

// ---- consumed payload shapes (producer-side JSON; see each service's events.go) ----

// auctionOpened mirrors auction-dutch's auction.opened. opened_at is the price
// clock origin; the notifier re-computes current_price(now) from these params.
type auctionOpened struct {
	AuctionID        string `json:"auction_id"`
	LotID            string `json:"lot_id"`
	Ceiling          money  `json:"ceiling"`
	Floor            money  `json:"floor"`
	DropStep         money  `json:"drop_step"`
	DropIntervalSecs uint32 `json:"drop_interval_secs"`
	OpenedAt         string `json:"opened_at"`
}

// auctionHammer mirrors auction-dutch's auction.hammer.
type auctionHammer struct {
	AuctionID   string `json:"auction_id"`
	LotID       string `json:"lot_id"`
	WinnerID    string `json:"winner_id"`
	HammerPrice money  `json:"hammer_price"`
	Premium     money  `json:"premium"`
	HammeredAt  string `json:"hammered_at"`
}

// auctionCompleted mirrors auction-dutch's auction.completed. final_state is an
// AuctionState name (COMPLETED / CANCELLED / ABORTED).
type auctionCompleted struct {
	AuctionID  string `json:"auction_id"`
	LotID      string `json:"lot_id"`
	FinalState string `json:"final_state"`
}

// bidPlaced mirrors auction-passive's bid.placed. The producer stamps `amount`
// (sealed) but the notifier MUST NOT surface it — only an activity toast.
type bidPlaced struct {
	AuctionID string `json:"auction_id"`
	BidderID  string `json:"bidder_id"`
	BidID     string `json:"bid_id"`
	Amount    money  `json:"amount"` // sealed — intentionally never broadcast
	PlacedAt  string `json:"placed_at"`
}

// auctionClosed mirrors auction-passive's auction.closed.
type auctionClosed struct {
	AuctionID string `json:"auction_id"`
	LotID     string `json:"lot_id"`
	Mode      string `json:"mode"`
	ClosedAt  string `json:"closed_at"`
}

// auctionWon mirrors auction-passive's auction.won. cleared_price is the Vickrey
// 2nd-price / UniqBid lowest-unique price (safe to reveal post-resolution).
type auctionWon struct {
	AuctionID    string `json:"auction_id"`
	LotID        string `json:"lot_id"`
	WinnerID     string `json:"winner_id"`
	ClearedPrice money  `json:"cleared_price"`
	Premium      money  `json:"premium"`
}

// escrowLocked mirrors escrow's escrow.locked (DEPOSIT_LOCKED/FULL_LOCKED/HELD).
type escrowLocked struct {
	TradeID       string `json:"trade_id"`
	ParticipantID string `json:"participant_id"`
	State         string `json:"state"`
	Amount        money  `json:"amount"`
}

// escrowReleased mirrors escrow's escrow.released.
type escrowReleased struct {
	TradeID       string `json:"trade_id"`
	SellerID      string `json:"seller_id"`
	Amount        money  `json:"amount"`
	AsVaultCredit bool   `json:"as_vault_credit"`
	CreditCents   int64  `json:"credit_cents,omitempty"`
}

// escrowParty mirrors escrow's forfeited/refunded family (same shape).
type escrowParty struct {
	TradeID       string `json:"trade_id"`
	ParticipantID string `json:"participant_id"`
	Amount        money  `json:"amount"`
}
