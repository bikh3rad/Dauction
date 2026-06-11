package repo

import (
	"application/internal/biz"
	"application/internal/entity"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// InsertOTP stores a hashed OTP for a mobile number. The raw code is never
// persisted. Existing un-consumed codes for the same number stay valid until
// they expire; ConsumeOTP only accepts the matching hash.
func (r *account) InsertOTP(
	ctx context.Context, mobile, codeHash, purpose string, expiresAt time.Time,
) error {
	ctx, span := r.tracer.Start(ctx, "account.InsertOTP")
	defer span.End()

	const query = `
		INSERT INTO mobile_otp (mobile_e164, code_hash, purpose, expires_at)
		VALUES ($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, query, mobile, codeHash, purpose, expiresAt)

	return err
}

// ConsumeOTP marks the matching active, unexpired OTP consumed and reports
// whether one was found. A miss increments attempts on the latest active code so
// brute force can be rate-limited in biz.
func (r *account) ConsumeOTP(ctx context.Context, mobile, codeHash string) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "account.ConsumeOTP")
	defer span.End()

	const query = `
		UPDATE mobile_otp SET consumed_at = NOW()
		WHERE id = (
			SELECT id FROM mobile_otp
			WHERE mobile_e164 = $1 AND code_hash = $2
			  AND consumed_at IS NULL AND expires_at > NOW()
			ORDER BY created_at DESC LIMIT 1
		)
		RETURNING id`
	var id int64
	err := r.db.QueryRowContext(ctx, query, mobile, codeHash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = r.db.ExecContext(ctx,
			`UPDATE mobile_otp SET attempts = attempts + 1
			 WHERE mobile_e164 = $1 AND consumed_at IS NULL`, mobile)

		return false, nil
	}

	return err == nil, err
}

// FindAccountByMobile returns the account bound to a mobile number, if any.
func (r *account) FindAccountByMobile(ctx context.Context, mobile string) (entity.Account, bool, error) {
	ctx, span := r.tracer.Start(ctx, "account.FindAccountByMobile")
	defer span.End()

	query := `SELECT ` + accountColumns + ` FROM account WHERE mobile_e164 = $1`
	a, err := scanAccount(r.db.QueryRowContext(ctx, query, mobile))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Account{}, false, nil
	}
	if err != nil {
		return entity.Account{}, false, err
	}
	if a.Roles, err = r.loadRoles(ctx, a.ID); err != nil {
		return entity.Account{}, false, err
	}

	return a, true, nil
}

// CreateMobileAccountTx inserts a new mobile-verified account and the
// account.registered outbox event in one transaction.
func (r *account) CreateMobileAccountTx(
	ctx context.Context, id uuid.UUID, mobile string, outbox entity.OutboxEvent,
) (entity.Account, error) {
	ctx, span := r.tracer.Start(ctx, "account.CreateMobileAccountTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Account{}, err
	}
	defer func() { _ = tx.Rollback() }()

	insert := `
		INSERT INTO account (id, mobile_e164, mobile_verified_at, status, tier, kyc_status)
		VALUES ($1, $2, NOW(), 'REGISTERED', 'GUEST', 'PENDING')
		RETURNING ` + accountColumns
	a, err := scanAccount(tx.QueryRowContext(ctx, insert, id, mobile))
	if err != nil {
		return entity.Account{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.Account{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Account{}, err
	}

	return a, nil
}

// UpsertOAuthTx links a social identity to an account, creating the account (and
// emitting account.registered) on first sight. Returns the account and whether it
// was newly created.
func (r *account) UpsertOAuthTx(
	ctx context.Context, provider, providerUserID, email string, newID uuid.UUID, outbox entity.OutboxEvent,
) (entity.Account, bool, error) {
	ctx, span := r.tracer.Start(ctx, "account.UpsertOAuthTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Account{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	// Existing link?
	var accountID uuid.UUID
	err = tx.QueryRowContext(ctx,
		`SELECT account_id FROM oauth_identity WHERE provider = $1 AND provider_user_id = $2`,
		provider, providerUserID).Scan(&accountID)

	created := false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// New account + link + registration event.
		accountID = newID
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO account (id, status, tier, kyc_status) VALUES ($1, 'REGISTERED', 'GUEST', 'PENDING')`,
			accountID); err != nil {
			return entity.Account{}, false, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO oauth_identity (account_id, provider, provider_user_id, email)
			 VALUES ($1, $2, $3, NULLIF($4, ''))`,
			accountID, provider, providerUserID, email); err != nil {
			return entity.Account{}, false, err
		}
		if err := insertOutbox(ctx, tx, outbox); err != nil {
			return entity.Account{}, false, err
		}
		created = true
	case err != nil:
		return entity.Account{}, false, err
	}

	query := `SELECT ` + accountColumns + ` FROM account WHERE id = $1`
	a, err := scanAccount(tx.QueryRowContext(ctx, query, accountID))
	if err != nil {
		return entity.Account{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Account{}, false, err
	}

	if a.Roles, err = r.loadRoles(ctx, accountID); err != nil {
		return entity.Account{}, false, err
	}

	return a, created, nil
}

// insertOutbox writes an outbox row inside an existing transaction (dedup on key).
func insertOutbox(ctx context.Context, tx *sql.Tx, o entity.OutboxEvent) error {
	const q = `
		INSERT INTO outbox (id, subject, idempotency_key, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING`
	_, err := tx.ExecContext(ctx, q, o.ID, o.Subject, o.IdempotencyKey, o.Payload)

	return err
}

// compile-time assertion that the account repo also satisfies RepositoryAuth.
var _ biz.RepositoryAuth = (*account)(nil)
