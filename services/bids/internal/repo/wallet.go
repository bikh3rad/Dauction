package repo

import (
	"application/internal/biz"
	"application/internal/datasource"
	"application/internal/entity"
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type wallet struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryWallet = (*wallet)(nil)

// NewWallet constructs the wallet repository (raw parameterized pgx SQL + OTel).
func NewWallet(logger *slog.Logger, db *datasource.PostgresDB) *wallet {
	return &wallet{
		logger: logger.With("layer", "WalletRepo"),
		tracer: otel.Tracer("WalletRepo"),
		db:     db,
	}
}

// GetWallet implements biz.RepositoryWallet: upsert a zero-balance row if absent,
// then return the current wallet. Read-through; the stored balance is authoritative.
func (r *wallet) GetWallet(ctx context.Context, accountID uuid.UUID) (entity.BidWallet, error) {
	ctx, span := r.tracer.Start(ctx, "wallet.GetWallet")
	defer span.End()

	const ensure = `
		INSERT INTO bid_wallet (account_id, balance_credits)
		VALUES ($1, 0)
		ON CONFLICT (account_id) DO NOTHING`

	if _, err := r.db.ExecContext(ctx, ensure, accountID); err != nil {
		r.logger.WarnContext(ctx, "ensure wallet failed", "error", err)

		return entity.BidWallet{}, err
	}

	const query = `SELECT account_id, balance_credits, updated_at FROM bid_wallet WHERE account_id = $1`

	var w entity.BidWallet
	if err := r.db.QueryRowContext(ctx, query, accountID).
		Scan(&w.AccountID, &w.BalanceCredits, &w.UpdatedAt); err != nil {
		return entity.BidWallet{}, err
	}

	return w, nil
}

// RecentPurchases implements biz.RepositoryWallet.
func (r *wallet) RecentPurchases(ctx context.Context, accountID uuid.UUID, limit int) ([]entity.BidPurchase, error) {
	ctx, span := r.tracer.Start(ctx, "wallet.RecentPurchases")
	defer span.End()

	const query = `
		SELECT id, account_id, package_id, credits_granted, usdc_charged_cents, created_at
		FROM bid_purchase
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.BidPurchase

	for rows.Next() {
		var p entity.BidPurchase
		if err := rows.Scan(&p.ID, &p.AccountID, &p.PackageID, &p.CreditsGranted, &p.USDCChargedCents, &p.CreatedAt); err != nil {
			r.logger.WarnContext(ctx, "scan purchase failed", "error", err)

			continue
		}

		out = append(out, p)
	}

	return out, rows.Err()
}

// RecentDebits implements biz.RepositoryWallet.
func (r *wallet) RecentDebits(ctx context.Context, accountID uuid.UUID, limit int) ([]entity.BidDebit, error) {
	ctx, span := r.tracer.Start(ctx, "wallet.RecentDebits")
	defer span.End()

	const query = `
		SELECT id, account_id, amount_credits, idempotency_key, auction_id, created_at
		FROM bid_debit
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.BidDebit

	for rows.Next() {
		var d entity.BidDebit
		if err := rows.Scan(&d.ID, &d.AccountID, &d.AmountCredits, &d.IdempotencyKey, &d.AuctionID, &d.CreatedAt); err != nil {
			r.logger.WarnContext(ctx, "scan debit failed", "error", err)

			continue
		}

		out = append(out, d)
	}

	return out, rows.Err()
}

// ListPackages implements biz.RepositoryWallet: the seeded package catalogue.
func (r *wallet) ListPackages(ctx context.Context) ([]entity.BidPackage, error) {
	ctx, span := r.tracer.Start(ctx, "wallet.ListPackages")
	defer span.End()

	const query = `
		SELECT id, credits, price_cents, best_value
		FROM bid_package
		ORDER BY price_cents DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.BidPackage

	for rows.Next() {
		var p entity.BidPackage
		if err := rows.Scan(&p.ID, &p.Credits, &p.PriceCents, &p.BestValue); err != nil {
			r.logger.WarnContext(ctx, "scan package failed", "error", err)

			continue
		}

		out = append(out, p)
	}

	return out, rows.Err()
}

// GetPackage implements biz.RepositoryWallet: one package or ErrResourceNotFound.
func (r *wallet) GetPackage(ctx context.Context, id string) (entity.BidPackage, error) {
	ctx, span := r.tracer.Start(ctx, "wallet.GetPackage")
	defer span.End()

	const query = `SELECT id, credits, price_cents, best_value FROM bid_package WHERE id = $1`

	var p entity.BidPackage
	err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Credits, &p.PriceCents, &p.BestValue)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.BidPackage{}, biz.ErrResourceNotFound
	}

	if err != nil {
		return entity.BidPackage{}, err
	}

	return p, nil
}

// GrantTx implements biz.RepositoryWallet: credit the wallet, write the purchase
// row, and the bids.purchased outbox row in ONE transaction. Idempotency is keyed
// on the outbox idempotency_key: if it already exists the purchase is a replay —
// nothing is granted and the current balance is returned.
func (r *wallet) GrantTx(
	ctx context.Context,
	purchase entity.BidPurchase,
	idempotencyKey string,
	outbox entity.OutboxEvent,
) (biz.PurchaseResult, error) {
	ctx, span := r.tracer.Start(ctx, "wallet.GrantTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return biz.PurchaseResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// The outbox unique idempotency_key is the dedup gate for the whole purchase.
	const outboxQuery = `
		INSERT INTO outbox (id, subject, idempotency_key, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING`

	res, err := tx.ExecContext(ctx, outboxQuery,
		outbox.ID, outbox.Subject, outbox.IdempotencyKey, outbox.Payload)
	if err != nil {
		return biz.PurchaseResult{}, err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		// Replay: this purchase already happened. Return the current balance and
		// grant nothing further (no double-credit).
		balance, berr := txWalletBalance(ctx, tx, purchase.AccountID)
		if berr != nil {
			return biz.PurchaseResult{}, berr
		}

		if cerr := tx.Commit(); cerr != nil {
			return biz.PurchaseResult{}, cerr
		}

		return biz.PurchaseResult{
			CreditsGranted:   purchase.CreditsGranted,
			USDCChargedCents: purchase.USDCChargedCents,
			Balance:          balance,
		}, nil
	}

	// Record the purchase (USDC charge + credit grant amounts — distinct units).
	const purchaseQuery = `
		INSERT INTO bid_purchase (id, account_id, package_id, credits_granted, usdc_charged_cents)
		VALUES ($1, $2, $3, $4, $5)`

	if _, err := tx.ExecContext(ctx, purchaseQuery,
		purchase.ID, purchase.AccountID, purchase.PackageID,
		purchase.CreditsGranted, purchase.USDCChargedCents); err != nil {
		return biz.PurchaseResult{}, err
	}

	// Atomically credit the wallet (upsert), returning the new balance.
	const creditQuery = `
		INSERT INTO bid_wallet (account_id, balance_credits, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (account_id)
		DO UPDATE SET balance_credits = bid_wallet.balance_credits + EXCLUDED.balance_credits,
		              updated_at = NOW()
		RETURNING balance_credits`

	var balance int64
	if err := tx.QueryRowContext(ctx, creditQuery, purchase.AccountID, purchase.CreditsGranted).
		Scan(&balance); err != nil {
		return biz.PurchaseResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return biz.PurchaseResult{}, err
	}

	return biz.PurchaseResult{
		CreditsGranted:   purchase.CreditsGranted,
		USDCChargedCents: purchase.USDCChargedCents,
		Balance:          balance,
	}, nil
}

// DebitTx implements biz.RepositoryWallet. Concurrency rule (CLAUDE.md §5): a
// conditional UPDATE `... WHERE account_id=$1 AND balance_credits >= $n` plus a
// UNIQUE debit-row insert, all in one tx. The bids.debited outbox row is built
// from the authoritative post-debit balance inside the tx and emitted only on a
// fresh burn. A replayed idempotency_key returns the original debit (burns
// nothing); an insufficient balance returns biz.ErrResourceInvalid.
func (r *wallet) DebitTx(
	ctx context.Context,
	debit entity.BidDebit,
) (biz.DebitResult, bool, error) {
	ctx, span := r.tracer.Start(ctx, "wallet.DebitTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return biz.DebitResult{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Idempotency gate: claim the key. ON CONFLICT DO NOTHING so a replay is
	//    detected without erroring.
	const claimQuery = `
		INSERT INTO bid_debit (id, account_id, amount_credits, idempotency_key, auction_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING`

	claimed, err := tx.ExecContext(ctx, claimQuery,
		debit.ID, debit.AccountID, debit.AmountCredits, debit.IdempotencyKey, debit.AuctionID)
	if err != nil {
		return biz.DebitResult{}, false, err
	}

	if affected, _ := claimed.RowsAffected(); affected == 0 {
		// Replay: return the ORIGINAL debit + the current balance, burn nothing.
		original, balance, rerr := txReplayedDebit(ctx, tx, debit.IdempotencyKey)
		if rerr != nil {
			return biz.DebitResult{}, false, rerr
		}

		if cerr := tx.Commit(); cerr != nil {
			return biz.DebitResult{}, false, cerr
		}

		return biz.DebitResult{Amount: original, Balance: balance}, false, nil
	}

	// 2. Conditional balance debit. Ensure a wallet row exists first so a debit on a
	//    never-seen account fails on insufficient funds (balance 0), not a missing row.
	const ensure = `
		INSERT INTO bid_wallet (account_id, balance_credits) VALUES ($1, 0)
		ON CONFLICT (account_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, ensure, debit.AccountID); err != nil {
		return biz.DebitResult{}, false, err
	}

	const debitQuery = `
		UPDATE bid_wallet
		SET balance_credits = balance_credits - $2, updated_at = NOW()
		WHERE account_id = $1 AND balance_credits >= $2
		RETURNING balance_credits`

	var balance int64
	err = tx.QueryRowContext(ctx, debitQuery, debit.AccountID, debit.AmountCredits).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		// Conditional UPDATE matched nothing => insufficient balance. Roll back the
		// claimed debit row by aborting the tx. "Out of credits" -> ErrResourceInvalid.
		return biz.DebitResult{}, false, biz.ErrResourceInvalid
	}

	if err != nil {
		return biz.DebitResult{}, false, err
	}

	// 3. Emit bids.debited with the authoritative post-debit balance, same tx.
	ob, err := biz.NewDebitedOutbox(debit.AccountID, debit.AmountCredits, balance, debit.IdempotencyKey)
	if err != nil {
		return biz.DebitResult{}, false, err
	}

	const outboxQuery = `
		INSERT INTO outbox (id, subject, idempotency_key, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING`
	if _, err := tx.ExecContext(ctx, outboxQuery, ob.ID, ob.Subject, ob.IdempotencyKey, ob.Payload); err != nil {
		return biz.DebitResult{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return biz.DebitResult{}, false, err
	}

	return biz.DebitResult{Amount: debit.AmountCredits, Balance: balance}, true, nil
}

// txWalletBalance reads the current balance inside a tx (0 if no row yet).
func txWalletBalance(ctx context.Context, tx *sql.Tx, accountID uuid.UUID) (int64, error) {
	const query = `SELECT COALESCE((SELECT balance_credits FROM bid_wallet WHERE account_id = $1), 0)`

	var balance int64
	if err := tx.QueryRowContext(ctx, query, accountID).Scan(&balance); err != nil {
		return 0, err
	}

	return balance, nil
}

// txReplayedDebit returns the original debit amount + the account's current balance
// for an already-recorded idempotency_key.
func txReplayedDebit(ctx context.Context, tx *sql.Tx, idempotencyKey string) (int64, int64, error) {
	const query = `
		SELECT d.amount_credits,
		       COALESCE((SELECT balance_credits FROM bid_wallet w WHERE w.account_id = d.account_id), 0)
		FROM bid_debit d
		WHERE d.idempotency_key = $1`

	var amount, balance int64
	if err := tx.QueryRowContext(ctx, query, idempotencyKey).Scan(&amount, &balance); err != nil {
		return 0, 0, err
	}

	return amount, balance, nil
}
