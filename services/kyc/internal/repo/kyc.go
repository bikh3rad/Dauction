package repo

import (
	"application/internal/biz"
	"application/internal/datasource"
	"application/internal/entity"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type kyc struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryKyc = (*kyc)(nil)

// NewKyc builds the KYC repository.
func NewKyc(logger *slog.Logger, db *datasource.PostgresDB) *kyc {
	return &kyc{
		logger: logger.With("layer", "KycRepo"),
		tracer: otel.Tracer("KycRepo"),
		db:     db,
	}
}

// CreateSubmission inserts a submission and its OTP challenge atomically.
func (r *kyc) CreateSubmission(ctx context.Context, s entity.Submission, c entity.OTPChallenge) error {
	ctx, span := r.tracer.Start(ctx, "CreateSubmission")
	defer span.End()

	return r.inTx(ctx, func(tx *sql.Tx) error {
		const subQ = `
			INSERT INTO kyc_submission
				(id, account_id, doc_type, doc_ref, phone, state, submitted_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`
		if _, err := tx.ExecContext(ctx, subQ,
			s.ID, s.AccountID, string(s.DocType), s.DocRef, s.Phone,
			string(s.State), s.SubmittedAt,
		); err != nil {
			return err
		}

		const chQ = `
			INSERT INTO otp_challenge
				(id, submission_id, phone, code_hash, attempts, verified, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
		if _, err := tx.ExecContext(ctx, chQ,
			c.ID, c.SubmissionID, c.Phone, c.CodeHash, c.Attempts,
			c.Verified, c.ExpiresAt, c.CreatedAt,
		); err != nil {
			return err
		}

		return nil
	})
}

// GetLatestSubmission returns the account's most recent submission.
func (r *kyc) GetLatestSubmission(ctx context.Context, accountID uuid.UUID) (entity.Submission, error) {
	const q = `
		SELECT id, account_id, doc_type, doc_ref, phone, state,
		       COALESCE(rejection_reason, ''), decided_by, submitted_at, decided_at
		FROM kyc_submission
		WHERE account_id = $1
		ORDER BY submitted_at DESC
		LIMIT 1`

	return r.scanSubmission(ctx, r.db.QueryRowContext(ctx, q, accountID))
}

// GetSubmission returns a submission by id.
func (r *kyc) GetSubmission(ctx context.Context, id uuid.UUID) (entity.Submission, error) {
	const q = `
		SELECT id, account_id, doc_type, doc_ref, phone, state,
		       COALESCE(rejection_reason, ''), decided_by, submitted_at, decided_at
		FROM kyc_submission
		WHERE id = $1`

	return r.scanSubmission(ctx, r.db.QueryRowContext(ctx, q, id))
}

// GetOpenChallenge returns the unverified challenge for a submission.
func (r *kyc) GetOpenChallenge(ctx context.Context, submissionID uuid.UUID) (entity.OTPChallenge, error) {
	const q = `
		SELECT id, submission_id, phone, code_hash, attempts, verified, expires_at, created_at
		FROM otp_challenge
		WHERE submission_id = $1 AND verified = FALSE
		ORDER BY created_at DESC
		LIMIT 1`

	var c entity.OTPChallenge

	err := r.db.QueryRowContext(ctx, q, submissionID).Scan(
		&c.ID, &c.SubmissionID, &c.Phone, &c.CodeHash,
		&c.Attempts, &c.Verified, &c.ExpiresAt, &c.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.OTPChallenge{}, biz.ErrResourceNotFound
	}

	if err != nil {
		return entity.OTPChallenge{}, err
	}

	return c, nil
}

// IncrementChallengeAttempts bumps the attempt counter.
func (r *kyc) IncrementChallengeAttempts(ctx context.Context, challengeID uuid.UUID) error {
	const q = `UPDATE otp_challenge SET attempts = attempts + 1 WHERE id = $1`

	_, err := r.db.ExecContext(ctx, q, challengeID)

	return err
}

// MarkVerifiedAndSubmit flags the challenge verified and advances the submission
// to SUBMITTED in one transaction.
func (r *kyc) MarkVerifiedAndSubmit(
	ctx context.Context,
	submissionID, challengeID uuid.UUID,
	submittedAt time.Time,
) error {
	ctx, span := r.tracer.Start(ctx, "MarkVerifiedAndSubmit")
	defer span.End()

	return r.inTx(ctx, func(tx *sql.Tx) error {
		const chQ = `UPDATE otp_challenge SET verified = TRUE WHERE id = $1`
		if _, err := tx.ExecContext(ctx, chQ, challengeID); err != nil {
			return err
		}

		const subQ = `
			UPDATE kyc_submission
			SET state = $1, submitted_at = $2
			WHERE id = $3 AND state = $4`
		res, err := tx.ExecContext(ctx, subQ,
			string(entity.SubmissionSubmitted), submittedAt,
			submissionID, string(entity.SubmissionStarted),
		)
		if err != nil {
			return err
		}

		if n, _ := res.RowsAffected(); n == 0 {
			return biz.ErrResourceInvalid
		}

		return nil
	})
}

// DecideSubmission writes the decision and an outbox row atomically.
func (r *kyc) DecideSubmission(ctx context.Context, s entity.Submission, outbox entity.OutboxEvent) error {
	ctx, span := r.tracer.Start(ctx, "DecideSubmission")
	defer span.End()

	return r.inTx(ctx, func(tx *sql.Tx) error {
		const subQ = `
			UPDATE kyc_submission
			SET state = $1, rejection_reason = $2, decided_by = $3, decided_at = $4
			WHERE id = $5 AND state = $6`
		res, err := tx.ExecContext(ctx, subQ,
			string(s.State), nullString(s.RejectionReason), s.DecidedBy, s.DecidedAt,
			s.ID, string(entity.SubmissionSubmitted),
		)
		if err != nil {
			return err
		}

		if n, _ := res.RowsAffected(); n == 0 {
			return biz.ErrResourceInvalid
		}

		const obQ = `
			INSERT INTO outbox (id, idempotency_key, subject, payload, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (idempotency_key) DO NOTHING`
		if _, err := tx.ExecContext(ctx, obQ,
			outbox.ID, outbox.IdempotencyKey, outbox.Subject, outbox.Payload, outbox.CreatedAt,
		); err != nil {
			return err
		}

		return nil
	})
}

// ListByState returns submissions in a state, newest first.
func (r *kyc) ListByState(ctx context.Context, state entity.SubmissionState) ([]entity.Submission, error) {
	const q = `
		SELECT id, account_id, doc_type, doc_ref, phone, state,
		       COALESCE(rejection_reason, ''), decided_by, submitted_at, decided_at
		FROM kyc_submission
		WHERE state = $1
		ORDER BY submitted_at ASC`

	rows, err := r.db.QueryContext(ctx, q, string(state))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.Submission

	for rows.Next() {
		var (
			s         entity.Submission
			docType   string
			st        string
			decidedBy sql.NullString
			decidedAt sql.NullTime
		)

		if err := rows.Scan(
			&s.ID, &s.AccountID, &docType, &s.DocRef, &s.Phone, &st,
			&s.RejectionReason, &decidedBy, &s.SubmittedAt, &decidedAt,
		); err != nil {
			return nil, err
		}

		s.DocType = entity.DocType(docType)
		s.State = entity.SubmissionState(st)
		applyDecided(&s, decidedBy, decidedAt)
		out = append(out, s)
	}

	return out, rows.Err()
}

// FetchUnpublished returns up to limit outbox rows not yet published.
func (r *kyc) FetchUnpublished(ctx context.Context, limit int) ([]entity.OutboxEvent, error) {
	const q = `
		SELECT id, idempotency_key, subject, payload, created_at, published_at
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.OutboxEvent

	for rows.Next() {
		var (
			ev          entity.OutboxEvent
			publishedAt sql.NullTime
		)

		if err := rows.Scan(
			&ev.ID, &ev.IdempotencyKey, &ev.Subject, &ev.Payload, &ev.CreatedAt, &publishedAt,
		); err != nil {
			return nil, err
		}

		if publishedAt.Valid {
			t := publishedAt.Time
			ev.PublishedAt = &t
		}

		out = append(out, ev)
	}

	return out, rows.Err()
}

// MarkPublished stamps an outbox row published.
func (r *kyc) MarkPublished(ctx context.Context, id uuid.UUID, publishedAt time.Time) error {
	const q = `UPDATE outbox SET published_at = $1 WHERE id = $2`

	_, err := r.db.ExecContext(ctx, q, publishedAt, id)

	return err
}

// scanSubmission scans a single submission row, mapping no-rows to NotFound.
func (r *kyc) scanSubmission(ctx context.Context, row *sql.Row) (entity.Submission, error) {
	_ = ctx

	var (
		s         entity.Submission
		docType   string
		st        string
		decidedBy sql.NullString
		decidedAt sql.NullTime
	)

	err := row.Scan(
		&s.ID, &s.AccountID, &docType, &s.DocRef, &s.Phone, &st,
		&s.RejectionReason, &decidedBy, &s.SubmittedAt, &decidedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Submission{}, biz.ErrResourceNotFound
	}

	if err != nil {
		return entity.Submission{}, err
	}

	s.DocType = entity.DocType(docType)
	s.State = entity.SubmissionState(st)
	applyDecided(&s, decidedBy, decidedAt)

	return s, nil
}

// inTx runs fn inside a transaction, committing on success and rolling back on error.
func (r *kyc) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()

		return err
	}

	return tx.Commit()
}

func applyDecided(s *entity.Submission, decidedBy sql.NullString, decidedAt sql.NullTime) {
	if decidedBy.Valid {
		if id, err := uuid.Parse(decidedBy.String); err == nil {
			s.DecidedBy = &id
		}
	}

	if decidedAt.Valid {
		t := decidedAt.Time
		s.DecidedAt = &t
	}
}

func nullString(s string) any {
	if s == "" {
		return nil
	}

	return s
}
