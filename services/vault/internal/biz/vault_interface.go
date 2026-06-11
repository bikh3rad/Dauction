package biz

import (
	"application/internal/entity"
	"context"

	"github.com/google/uuid"
)

// VaultView is the read model returned by GET /apis/vault: the caller's objects
// plus their derived Vault-Credit balance (USDC cents).
type VaultView struct {
	Objects            []entity.VaultObject
	CreditBalanceCents int64
}

// ListRequest is the validated input for listing an object to auction. Mode is
// the engine; DurationDays is required for timed (VICKREY/UNIQBID) and forbidden
// for DUTCH (CLAUDE.md §6) — the use case enforces this matrix. CategoryCode,
// Translations (4 langs, back-filled from PrimaryLang) and ImageRefs (≤7) capture
// the listing content (§3/§4/§5).
type ListRequest struct {
	Mode         entity.AuctionMode
	DurationDays int // 0 when DUTCH; 2/5/7 when timed
	CategoryCode string
	PrimaryLang  string
	Translations []entity.ObjectTranslation
	ImageRefs    []string
}

// UsecaseVault is the vault use case consumed by handlers and event consumers.
type UsecaseVault interface {
	// View returns the caller's objects and Vault-Credit balance.
	View(ctx context.Context, owner uuid.UUID) (VaultView, error)
	// AddObject creates a new IN_VAULT object owned by owner.
	AddObject(ctx context.Context, owner uuid.UUID, title, description string, appraisedValueCents int64) (entity.VaultObject, error)
	// List moves an owned IN_VAULT object to APPRAISING and emits object.listed.
	// Owner mismatch -> ErrResourceAccessDenied; bad duration/mode -> ErrResourceInvalid.
	List(ctx context.Context, owner, objectID uuid.UUID, req ListRequest) (entity.VaultObject, error)
	// Buyback takes instant buyback on an owned IN_VAULT object: CASH pays 50%
	// of appraised value (settled off-ledger), CREDIT credits 85% to the
	// vault-credit ledger and emits credit.changed. Object -> BOUGHT_BACK.
	Buyback(ctx context.Context, owner, objectID uuid.UUID, mode entity.BuybackMode) (BuybackResult, error)

	// SettleAuctionCompleted consumes auction.completed: an owned IN_AUCTION
	// object becomes SOLD. If asVaultCredit is set, the seller's release amount
	// is appended to the ledger and credit.changed is emitted. Idempotent on
	// idempotencyKey via the inbox.
	SettleAuctionCompleted(ctx context.Context, in AuctionCompletedInput) error
}

// BuybackResult reports the outcome of an instant buyback.
type BuybackResult struct {
	Object       entity.VaultObject
	Mode         entity.BuybackMode
	PayoutCents  int64 // 50% (cash) or 85% (credit) of appraised value
	BalanceCents int64 // resulting Vault-Credit balance (only meaningful for CREDIT)
}

// AuctionCompletedInput is the decoded auction.completed event the consumer
// applies. ReleaseCents/AsVaultCredit describe the seller's settlement; they are
// only honoured when AsVaultCredit is true (a Vault-Credit release).
type AuctionCompletedInput struct {
	ObjectID       uuid.UUID
	AsVaultCredit  bool
	ReleaseCents   int64
	IdempotencyKey string
}

// RepositoryVault is the persistence seam (implemented by internal/repo, mocked
// in tests). State mutations that emit an event do so atomically with an outbox
// row via the *Tx methods (the outbox pattern, CLAUDE.md §0).
type RepositoryVault interface {
	// GetObject returns the object or ErrResourceNotFound.
	GetObject(ctx context.Context, objectID uuid.UUID) (entity.VaultObject, error)
	// ListObjects returns all objects owned by owner (newest first).
	ListObjects(ctx context.Context, owner uuid.UUID) ([]entity.VaultObject, error)
	// InsertObject persists a new object and returns it.
	InsertObject(ctx context.Context, obj entity.VaultObject) (entity.VaultObject, error)
	// CreditBalance returns SUM(delta_cents) for account (0 when empty).
	CreditBalance(ctx context.Context, account uuid.UUID) (int64, error)

	// TransitionTx atomically moves objectID from `from` to `to` (conditional
	// UPDATE) and writes the outbox row in one transaction. If the row is not in
	// `from`, returns ErrResourceInvalid. Returns the updated object.
	TransitionTx(ctx context.Context, objectID uuid.UUID, from, to entity.ObjectState, outbox entity.OutboxEvent) (entity.VaultObject, error)

	// ListWithDetailsTx atomically transitions an object from `from` to `to`,
	// writes its category + replaces its translations + media, and writes the
	// object.listed outbox row — all in one transaction. Not in `from` ->
	// ErrResourceInvalid. Returns the updated object.
	ListWithDetailsTx(ctx context.Context, objectID uuid.UUID, from, to entity.ObjectState, details entity.ListingDetails, outbox entity.OutboxEvent) (entity.VaultObject, error)

	// BuybackTx atomically moves objectID IN_VAULT -> BOUGHT_BACK. When entry is
	// non-nil it is appended to the vault-credit ledger and buildOutbox(balance)
	// is invoked INSIDE the transaction (after the new balance is known) to build
	// the credit.changed outbox row, which is written in the same tx. Pass a nil
	// entry (and nil buildOutbox) for CASH buyback (no ledger row, no event).
	// Returns the updated object and the resulting credit balance.
	BuybackTx(ctx context.Context, objectID uuid.UUID, entry *entity.VaultCreditEntry, buildOutbox OutboxBuilder) (entity.VaultObject, int64, error)

	// SettleSoldTx atomically marks objectID IN_AUCTION -> SOLD and records the
	// consumed event (inbox). When entry is non-nil it appends a ledger row and
	// buildOutbox(balance) is invoked inside the tx to write a credit.changed
	// outbox row. A duplicate inboxKey yields ErrResourceExists. Returns the
	// resulting credit balance.
	SettleSoldTx(ctx context.Context, objectID uuid.UUID, inboxKey string, entry *entity.VaultCreditEntry, buildOutbox OutboxBuilder) (int64, error)
}

// OutboxBuilder builds a credit.changed outbox row from the post-write balance.
// It is invoked by the repo INSIDE the transaction so the event commits with the
// ledger row. A nil builder means "no event".
type OutboxBuilder func(balanceCents int64) (entity.OutboxEvent, error)
