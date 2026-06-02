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

type account struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryAccount = (*account)(nil)

// NewAccount constructs the account repository (raw parameterized pgx SQL with an
// OTel tracer, like the template's placeholder repo).
func NewAccount(logger *slog.Logger, db *datasource.PostgresDB) *account {
	return &account{
		logger: logger.With("layer", "AccountRepo"),
		tracer: otel.Tracer("AccountRepo"),
		db:     db,
	}
}

// Get implements biz.RepositoryAccount.
func (r *account) Get(ctx context.Context, id uuid.UUID) (entity.Account, error) {
	ctx, span := r.tracer.Start(ctx, "account.Get")
	defer span.End()

	const query = `SELECT id, tier, kyc_status, created_at, updated_at FROM account WHERE id = $1`

	var a entity.Account
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&a.ID, &a.Tier, &a.KycStatus, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Account{}, biz.ErrResourceNotFound
	}

	if err != nil {
		r.logger.WarnContext(ctx, "get account failed", "error", err)

		return entity.Account{}, err
	}

	return a, nil
}

// EnsureExists implements biz.RepositoryAccount: upsert a GUEST/PENDING row if
// absent, then return the current account.
func (r *account) EnsureExists(ctx context.Context, id uuid.UUID) (entity.Account, error) {
	ctx, span := r.tracer.Start(ctx, "account.EnsureExists")
	defer span.End()

	const query = `
		INSERT INTO account (id, tier, kyc_status)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING`

	if _, err := r.db.ExecContext(ctx, query, id, entity.TierGuest, entity.KycPending); err != nil {
		r.logger.WarnContext(ctx, "ensure account failed", "error", err)

		return entity.Account{}, err
	}

	return r.Get(ctx, id)
}

// SetTierTx implements biz.RepositoryAccount: persist the new tier AND the
// outbox event row in one transaction (the outbox pattern — a tier change is
// never published without being recorded, and vice-versa).
func (r *account) SetTierTx(
	ctx context.Context,
	id uuid.UUID,
	to entity.Tier,
	outbox entity.OutboxEvent,
) (entity.Account, error) {
	ctx, span := r.tracer.Start(ctx, "account.SetTierTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Account{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const updateQuery = `
		UPDATE account SET tier = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, tier, kyc_status, created_at, updated_at`

	var a entity.Account
	if err := tx.QueryRowContext(ctx, updateQuery, to, id).
		Scan(&a.ID, &a.Tier, &a.KycStatus, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Account{}, biz.ErrResourceNotFound
		}

		return entity.Account{}, err
	}

	const outboxQuery = `
		INSERT INTO outbox (id, subject, idempotency_key, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING`

	if _, err := tx.ExecContext(ctx, outboxQuery,
		outbox.ID, outbox.Subject, outbox.IdempotencyKey, outbox.Payload); err != nil {
		return entity.Account{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Account{}, err
	}

	return a, nil
}

// SetKycTx implements biz.RepositoryAccount: mark the consumed event and update
// the kyc mirror in one transaction. A duplicate inboxKey yields
// biz.ErrResourceExists so the use case can treat it as an idempotent no-op.
func (r *account) SetKycTx(ctx context.Context, id uuid.UUID, status entity.KycState, inboxKey string) error {
	ctx, span := r.tracer.Start(ctx, "account.SetKycTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const inboxQuery = `
		INSERT INTO consumed_event (idempotency_key) VALUES ($1)
		ON CONFLICT (idempotency_key) DO NOTHING`

	res, err := tx.ExecContext(ctx, inboxQuery, inboxKey)
	if err != nil {
		return err
	}

	if affected, _ := res.RowsAffected(); affected == 0 {
		return biz.ErrResourceExists
	}

	const updateQuery = `UPDATE account SET kyc_status = $1, updated_at = NOW() WHERE id = $2`
	if _, err := tx.ExecContext(ctx, updateQuery, status, id); err != nil {
		return err
	}

	return tx.Commit()
}

// MarkConsumed implements biz.RepositoryAccount: insert inboxKey if absent.
// Returns true when newly inserted (event not seen before).
func (r *account) MarkConsumed(ctx context.Context, inboxKey string) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "account.MarkConsumed")
	defer span.End()

	const query = `
		INSERT INTO consumed_event (idempotency_key) VALUES ($1)
		ON CONFLICT (idempotency_key) DO NOTHING`

	res, err := r.db.ExecContext(ctx, query, inboxKey)
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}
