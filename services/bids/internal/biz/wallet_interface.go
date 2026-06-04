package biz

import (
	"application/internal/entity"
	"context"

	"github.com/google/uuid"
)

// WalletView is the read-through wallet projection returned by GET /apis/bids/wallet:
// the authoritative stored balance plus recent purchases and debits for the caller.
type WalletView struct {
	Wallet    entity.BidWallet
	Purchases []entity.BidPurchase
	Debits    []entity.BidDebit
}

// PurchaseResult is the outcome of a successful credit-package purchase.
type PurchaseResult struct {
	CreditsGranted   int64 // whole bid credits added
	USDCChargedCents int64 // USDC cents charged
	Balance          int64 // resulting wallet balance (credits)
}

// DebitResult is the outcome of a debit-on-bid. On an idempotent replay it carries
// the ORIGINAL debit's resulting balance (nothing is burned a second time).
type DebitResult struct {
	Amount  int64 // credits debited by the original write
	Balance int64 // wallet balance after the (original) debit
}

// UsecaseWallet is the bid-credit use case consumed by handlers and the internal
// debit endpoint (CLAUDE.md §5). Money is int64 USDC cents; credits are int64
// whole credits — never mixed.
type UsecaseWallet interface {
	// Wallet returns the caller's read-through balance plus recent activity.
	Wallet(ctx context.Context, accountID uuid.UUID, limit int) (WalletView, error)
	// Packages lists the seeded credit packages (public catalogue).
	Packages(ctx context.Context) ([]entity.BidPackage, error)
	// Buy records a package purchase: the USDC charge and the credit grant commit
	// atomically with a bids.purchased outbox row. Unknown package -> ErrResourceInvalid.
	// Idempotent on idempotencyKey: a replay returns the original grant, no double-credit.
	Buy(ctx context.Context, accountID uuid.UUID, packageID, idempotencyKey string) (PurchaseResult, error)
	// Debit spends `amount` credits for a bid. Insufficient balance -> ErrResourceInvalid
	// ("out of credits"). MUST be idempotent on idempotencyKey: a replay returns the
	// original debit and burns nothing. Emits bids.debited in the same tx via outbox.
	Debit(ctx context.Context, accountID uuid.UUID, amount int64, idempotencyKey, auctionID string) (DebitResult, error)
}

// RepositoryWallet is the persistence seam (implemented by internal/repo, mocked in
// tests). All mutations that emit an event do so atomically with an outbox row.
type RepositoryWallet interface {
	// GetWallet returns the wallet, creating a zero-balance row on first sight so a
	// caller always has a read model. Read-through; never recomputed.
	GetWallet(ctx context.Context, accountID uuid.UUID) (entity.BidWallet, error)
	// RecentPurchases / RecentDebits feed the wallet view.
	RecentPurchases(ctx context.Context, accountID uuid.UUID, limit int) ([]entity.BidPurchase, error)
	RecentDebits(ctx context.Context, accountID uuid.UUID, limit int) ([]entity.BidDebit, error)

	// ListPackages returns the seeded package catalogue.
	ListPackages(ctx context.Context) ([]entity.BidPackage, error)
	// GetPackage returns one package or ErrResourceNotFound.
	GetPackage(ctx context.Context, id string) (entity.BidPackage, error)

	// GrantTx credits the wallet (upsert), writes the bid_purchase row, and the
	// bids.purchased outbox row in ONE transaction. Idempotent on purchase.ID's
	// idempotency surface: a duplicate idempotencyKey returns the original result
	// (resulting balance) and grants nothing further.
	GrantTx(ctx context.Context, purchase entity.BidPurchase, idempotencyKey string, outbox entity.OutboxEvent) (PurchaseResult, error)

	// DebitTx performs a conditional balance UPDATE plus a unique debit-row insert
	// plus the bids.debited outbox row in ONE transaction (CLAUDE.md §5 concurrency
	// rule). The outbox row is built INSIDE the tx (via biz.NewDebitedOutbox) once
	// the post-debit balance is known. It returns:
	//   - (result, true, nil)  when this is the first time the key is seen and the
	//     balance covered the debit (a fresh burn),
	//   - (originalResult, false, nil) when idempotencyKey was already used (replay;
	//     returns the original debit, burns nothing, emits nothing),
	//   - (_, _, ErrResourceInvalid) when the balance is insufficient (out of credits).
	DebitTx(ctx context.Context, debit entity.BidDebit) (result DebitResult, fresh bool, err error)
}
