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

// accountColumns is the canonical projection used everywhere an Account is read.
// Nullable auth columns are COALESCEd so the entity scan stays string-typed.
const accountColumns = `
	id, COALESCE(handle, ''), COALESCE(mobile_e164, ''),
	mobile_verified_at IS NOT NULL, tier, kyc_status, status, created_at, updated_at`

// scanAccount scans the accountColumns projection into an entity.Account (roles
// are loaded separately).
func scanAccount(row interface{ Scan(...any) error }) (entity.Account, error) {
	var a entity.Account
	err := row.Scan(&a.ID, &a.Handle, &a.MobileE164, &a.MobileVerified,
		&a.Tier, &a.KycStatus, &a.Status, &a.CreatedAt, &a.UpdatedAt)

	return a, err
}

// Get implements biz.RepositoryAccount.
func (r *account) Get(ctx context.Context, id uuid.UUID) (entity.Account, error) {
	ctx, span := r.tracer.Start(ctx, "account.Get")
	defer span.End()

	query := `SELECT ` + accountColumns + ` FROM account WHERE id = $1`

	a, err := scanAccount(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Account{}, biz.ErrResourceNotFound
	}

	if err != nil {
		r.logger.WarnContext(ctx, "get account failed", "error", err)

		return entity.Account{}, err
	}

	roles, err := r.loadRoles(ctx, id)
	if err != nil {
		return entity.Account{}, err
	}
	a.Roles = roles

	return a, nil
}

// loadRoles returns the elevated roles held by an account (empty for plain USER).
func (r *account) loadRoles(ctx context.Context, id uuid.UUID) ([]entity.Role, error) {
	const query = `SELECT role FROM account_role WHERE account_id = $1 ORDER BY role`

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var roles []entity.Role
	for rows.Next() {
		var role entity.Role
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, rows.Err()
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

	updateQuery := `
		UPDATE account SET tier = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING ` + accountColumns

	a, err := scanAccount(tx.QueryRowContext(ctx, updateQuery, to, id))
	if err != nil {
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

	if a.Roles, err = r.loadRoles(ctx, id); err != nil {
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
