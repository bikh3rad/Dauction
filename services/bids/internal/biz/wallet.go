package biz

import (
	"application/internal/entity"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// defaultActivityLimit bounds the recent-activity lists in the wallet view.
const defaultActivityLimit = 20

type wallet struct {
	logger *slog.Logger
	repo   RepositoryWallet
}

var _ UsecaseWallet = (*wallet)(nil)

// NewWallet constructs the bid-credit use case.
func NewWallet(logger *slog.Logger, repo RepositoryWallet) *wallet {
	return &wallet{
		logger: logger.With("layer", "WalletUsecase"),
		repo:   repo,
	}
}

// Wallet returns the caller's read-through balance plus recent purchases/debits.
// The balance is the stored authoritative value — never recomputed here (CLAUDE.md §5).
func (uc *wallet) Wallet(ctx context.Context, accountID uuid.UUID, limit int) (WalletView, error) {
	if limit <= 0 {
		limit = defaultActivityLimit
	}

	w, err := uc.repo.GetWallet(ctx, accountID)
	if err != nil {
		return WalletView{}, err
	}

	purchases, err := uc.repo.RecentPurchases(ctx, accountID, limit)
	if err != nil {
		return WalletView{}, err
	}

	debits, err := uc.repo.RecentDebits(ctx, accountID, limit)
	if err != nil {
		return WalletView{}, err
	}

	return WalletView{Wallet: w, Purchases: purchases, Debits: debits}, nil
}

// Packages lists the seeded credit packages.
func (uc *wallet) Packages(ctx context.Context) ([]entity.BidPackage, error) {
	return uc.repo.ListPackages(ctx)
}

// Buy records a package purchase. The USDC charge and the credit grant commit
// atomically with a bids.purchased outbox row. Unknown package -> ErrResourceInvalid.
// Idempotent on idempotencyKey so a retried buy never double-credits.
func (uc *wallet) Buy(
	ctx context.Context,
	accountID uuid.UUID,
	packageID, idempotencyKey string,
) (PurchaseResult, error) {
	logger := uc.logger.With("method", "Buy", "account", accountID, "package", packageID)

	pkg, err := uc.repo.GetPackage(ctx, packageID)
	if err != nil {
		// An unknown package is a client error, not a 404 on the wallet resource.
		logger.WarnContext(ctx, "unknown package", "error", err)

		return PurchaseResult{}, fmt.Errorf("%w: unknown package %q", ErrResourceInvalid, packageID)
	}

	// Producer-stable idempotency: caller-supplied key, else a synthetic one per
	// account+package (best-effort — callers SHOULD pass a key).
	key := idempotencyKey
	if key == "" {
		key = fmt.Sprintf("bids:purchase:%s:%s:%d", accountID, packageID, time.Now().UTC().UnixNano())
	}

	purchase := entity.BidPurchase{
		ID:               uuid.New(),
		AccountID:        accountID,
		PackageID:        pkg.ID,
		CreditsGranted:   pkg.Credits,
		USDCChargedCents: pkg.PriceCents,
	}

	outbox, err := newPurchasedOutbox(accountID, pkg.ID, pkg.Credits, pkg.PriceCents, key)
	if err != nil {
		return PurchaseResult{}, err
	}

	res, err := uc.repo.GrantTx(ctx, purchase, key, outbox)
	if err != nil {
		return PurchaseResult{}, err
	}

	logger.InfoContext(ctx, "credits granted", "credits", res.CreditsGranted, "balance", res.Balance)

	return res, nil
}

// Debit spends `amount` credits for a bid. It is the sync call auction-passive makes
// BEFORE recording a bid. Insufficient balance -> ErrResourceInvalid ("out of
// credits"). Idempotent on idempotencyKey: a replay returns the original debit and
// burns nothing. Emits bids.debited in the same tx via outbox.
func (uc *wallet) Debit(
	ctx context.Context,
	accountID uuid.UUID,
	amount int64,
	idempotencyKey, auctionID string,
) (DebitResult, error) {
	logger := uc.logger.With("method", "Debit", "account", accountID, "key", idempotencyKey)

	if amount <= 0 {
		return DebitResult{}, fmt.Errorf("%w: debit amount must be positive (got %d)", ErrResourceInvalid, amount)
	}

	if idempotencyKey == "" {
		return DebitResult{}, fmt.Errorf("%w: debit requires an idempotency key", ErrResourceInvalid)
	}

	debit := entity.BidDebit{
		ID:             uuid.New(),
		AccountID:      accountID,
		AmountCredits:  amount,
		IdempotencyKey: idempotencyKey,
		AuctionID:      auctionID,
	}

	// The post-debit balance is only known inside the tx, so the repo builds the
	// bids.debited outbox row (via biz.NewDebitedOutbox) once the conditional UPDATE
	// has returned the authoritative balance, and emits only on a fresh burn.
	res, fresh, err := uc.repo.DebitTx(ctx, debit)
	if err != nil {
		// Insufficient balance surfaces as ErrResourceInvalid from the repo.
		logger.WarnContext(ctx, "debit failed", "error", err)

		return DebitResult{}, err
	}

	if fresh {
		logger.InfoContext(ctx, "credit debited", "amount", amount, "balance", res.Balance)
	} else {
		logger.InfoContext(ctx, "idempotent debit replay; nothing burned", "balance", res.Balance)
	}

	return res, nil
}
