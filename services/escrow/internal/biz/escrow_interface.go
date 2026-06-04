package biz

import (
	"application/internal/entity"
	"context"

	"github.com/google/uuid"
)

// TradeView is the read model returned by GET /apis/escrow/{tradeId}: the trade
// head plus the derived per-participant balances and the conservation summary.
type TradeView struct {
	Trade        entity.EscrowTrade
	Balances     []entity.ParticipantBalance
	Conservation entity.Conservation
}

// LockRequest is the decoded escrow.lock_requested input from auction-dutch. It
// creates (or advances) a Dutch trade and writes a DEPOSIT_LOCK (reserve) or
// FULL_LOCK ledger row, emitting escrow.locked.
type LockRequest struct {
	TradeID         uuid.UUID
	LotID           uuid.UUID
	BuyerAccountID  uuid.UUID
	SellerAccountID uuid.UUID
	State           entity.EscrowState // DEPOSIT_LOCKED or FULL_LOCKED (the target)
	AmountCents     int64              // the increment to lock this step
	IdempotencyKey  string
}

// HammerInput is the decoded auction.hammer input (Dutch): the FULL_LOCKED winner
// trade moves to HELD; losing reservations are marked for refund elsewhere.
type HammerInput struct {
	TradeID          uuid.UUID
	WinnerID         uuid.UUID
	HammerPriceCents int64
	PremiumCents     int64
	IdempotencyKey   string
}

// WonInput is the decoded auction.won input (passive): a trade is created in HELD-
// pending state (UNLOCKED + funding_deadline = now+24h); the winner must fund the
// cleared price + premium within the window.
type WonInput struct {
	TradeID           uuid.UUID
	LotID             uuid.UUID
	WinnerID          uuid.UUID
	SellerAccountID   uuid.UUID
	ClearedPriceCents int64
	PremiumCents      int64
	IdempotencyKey    string
}

// DisputeInput is the decoded dispute.resolved input: apply the ruling to a
// HELD/DISPUTED trade.
type DisputeInput struct {
	TradeID        uuid.UUID
	Ruling         entity.DisputeRuling
	BuyerCents     int64 // SPLIT: buyer refund portion (informational; engine recomputes)
	SellerCents    int64 // SPLIT: seller release portion (informational; engine recomputes)
	IdempotencyKey string
}

// UsecaseEscrow is the escrow use case consumed by handlers and event consumers.
// It owns the funds-ledger state machine and the conservation invariant (§4).
type UsecaseEscrow interface {
	// Get returns the trade head + derived per-participant balances.
	Get(ctx context.Context, tradeID uuid.UUID) (TradeView, error)

	// Fund records the winner funding their obligation. Within the funding
	// deadline it moves the trade to HELD (writing HOLD ledger rows); past the
	// deadline it FORFEITs. amountCents must equal the obligation exactly
	// (mismatch -> ErrResourceInvalid). Double-fund -> ErrResourceInvalid.
	Fund(ctx context.Context, tradeID, caller uuid.UUID, amountCents int64) (entity.EscrowTrade, error)

	// Confirm records buyer delivery confirmation: HELD -> RELEASED. The held pot
	// is split into RELEASE (to seller) + FEE + PREMIUM + INSPECTOR_FEE carve-outs
	// (conservation preserved). releaseMode CASH pays 100%; VAULT_CREDIT records a
	// 110% credit instruction. Blocked if DISPUTED. Emits escrow.released.
	Confirm(ctx context.Context, tradeID, caller uuid.UUID, mode entity.ReleaseMode) (entity.EscrowTrade, error)

	// Refund is the admin/loser refund: returns a participant's locked funds
	// (REFUND ledger row, negative) and moves the trade to REFUNDED. Used for
	// losing reservations and manual corrections.
	Refund(ctx context.Context, tradeID, participant uuid.UUID) (entity.EscrowTrade, error)

	// Forfeit is the admin/manual forfeit: a HELD/locked winner who missed the
	// funding window forfeits; funds move to FORFEITED. Emits escrow.forfeited.
	Forfeit(ctx context.Context, tradeID uuid.UUID) (entity.EscrowTrade, error)

	// LockRequested consumes escrow.lock_requested (Dutch reserve / full-lock):
	// idempotent create-or-advance + DEPOSIT_LOCK/FULL_LOCK ledger row + emits
	// escrow.locked.
	LockRequested(ctx context.Context, in LockRequest) error

	// Hammer consumes auction.hammer (Dutch): FULL_LOCKED winner -> HELD.
	Hammer(ctx context.Context, in HammerInput) error

	// Won consumes auction.won (passive): create the trade UNLOCKED with a 24h
	// funding deadline; the winner funds into HELD via Fund.
	Won(ctx context.Context, in WonInput) error

	// DisputeResolved consumes dispute.resolved: apply REFUND_BUYER /
	// RELEASE_SELLER / SPLIT to a HELD/DISPUTED trade.
	DisputeResolved(ctx context.Context, in DisputeInput) error
}

// RepositoryEscrow is the persistence seam (implemented by internal/repo, mocked
// in tests). `escrow` is the SOLE writer of escrow rows; every state mutation
// appends balancing ledger rows and (optionally) an outbox row in ONE tx.
type RepositoryEscrow interface {
	// GetTrade returns the trade head or ErrResourceNotFound.
	GetTrade(ctx context.Context, tradeID uuid.UUID) (entity.EscrowTrade, error)
	// ListEntries returns all ledger rows for a trade (oldest first).
	ListEntries(ctx context.Context, tradeID uuid.UUID) ([]entity.LedgerEntry, error)
	// Balances returns the derived per-participant balances for a trade.
	Balances(ctx context.Context, tradeID uuid.UUID) ([]entity.ParticipantBalance, error)

	// CreateTradeTx inserts a new trade head + its initial ledger rows + optional
	// outbox row in one tx. A duplicate trade id -> ErrResourceExists.
	CreateTradeTx(ctx context.Context, trade entity.EscrowTrade, entries []entity.LedgerEntry, outbox *entity.OutboxEvent) error

	// TransitionTx atomically advances a trade from `from` to `to` (conditional
	// UPDATE WHERE state = from), appends entries, optionally sets release_mode /
	// funding_deadline, and writes the optional outbox row — all in one tx. A row
	// not in `from` -> ErrResourceInvalid. Returns the updated trade.
	TransitionTx(ctx context.Context, tradeID uuid.UUID, from, to entity.EscrowState, upd TradeUpdate, entries []entity.LedgerEntry, outbox *entity.OutboxEvent) (entity.EscrowTrade, error)

	// MarkConsumed records inboxKey if absent; true when newly inserted (the
	// event has not been processed before). Used to make consumers idempotent
	// where the state change is conditional (e.g. already-advanced trades).
	MarkConsumed(ctx context.Context, inboxKey string) (bool, error)
}

// TradeUpdate carries the optional head-column changes a transition applies
// alongside the state flip. A nil field is left unchanged.
type TradeUpdate struct {
	ReleaseMode *entity.ReleaseMode // set on RELEASE
}

// RepositoryOutbox is the persistence seam for the transactional outbox relay.
type RepositoryOutbox interface {
	FetchUnpublished(ctx context.Context, limit int) ([]entity.OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
}

// EventPublisher relays an outbox payload to the message bus (NATS/JetStream).
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}
