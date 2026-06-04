package repo

import (
	"application/internal/biz"
	"application/internal/datasource"
	"application/internal/entity"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type dispute struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryDispute = (*dispute)(nil)

// NewDispute constructs the dispute repository (raw parameterized pgx SQL with an
// OTel tracer, like the template's placeholder repo).
func NewDispute(logger *slog.Logger, db *datasource.PostgresDB) *dispute {
	return &dispute{
		logger: logger.With("layer", "DisputeRepo"),
		tracer: otel.Tracer("DisputeRepo"),
		db:     db,
	}
}

const disputeColumns = `id, trade_id, claimant_account_id, respondent_account_id, reason_code,
	state, ruling, evidence_ref, ruled_by, created_at, resolved_at`

// scanDispute scans one dispute row in disputeColumns order.
func scanDispute(s interface {
	Scan(dest ...any) error
}) (entity.Dispute, error) {
	var (
		d      entity.Dispute
		ruling sql.NullString
		ruled  uuid.NullUUID
	)

	if err := s.Scan(
		&d.ID, &d.TradeID, &d.ClaimantAccountID, &d.RespondentAccountID, &d.ReasonCode,
		&d.State, &ruling, &d.EvidenceRef, &ruled, &d.CreatedAt, &d.ResolvedAt,
	); err != nil {
		return entity.Dispute{}, err
	}

	if ruling.Valid {
		r := entity.Ruling(ruling.String)
		d.Ruling = &r
	}

	if ruled.Valid {
		id := ruled.UUID
		d.RuledBy = &id
	}

	return d, nil
}

// CreateTx inserts the OPEN dispute, the OPENED audit row, and the dispute.opened
// outbox row in one transaction. The partial unique index
// uq_dispute_open_per_trade guarantees only one non-terminal dispute per trade;
// a violation surfaces as ErrResourceExists.
func (r *dispute) CreateTx(
	ctx context.Context,
	d entity.Dispute,
	audit entity.DisputeEvent,
	outbox entity.OutboxEvent,
) (entity.Dispute, error) {
	ctx, span := r.tracer.Start(ctx, "dispute.CreateTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Dispute{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Pre-check is belt-and-suspenders; the partial unique index is authoritative.
	const existsQuery = `
		SELECT 1 FROM dispute
		WHERE trade_id = $1 AND state IN ('OPEN', 'UNDER_REVIEW')
		LIMIT 1`

	var sentinel int
	switch err := tx.QueryRowContext(ctx, existsQuery, d.TradeID).Scan(&sentinel); {
	case err == nil:
		return entity.Dispute{}, biz.ErrResourceExists
	case errors.Is(err, sql.ErrNoRows):
		// no active dispute — proceed
	default:
		return entity.Dispute{}, err
	}

	const insertQuery = `
		INSERT INTO dispute (id, trade_id, claimant_account_id, respondent_account_id,
			reason_code, state, evidence_ref, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	if _, err := tx.ExecContext(ctx, insertQuery,
		d.ID, d.TradeID, d.ClaimantAccountID, d.RespondentAccountID,
		d.ReasonCode, d.State, d.EvidenceRef, d.CreatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return entity.Dispute{}, biz.ErrResourceExists
		}

		return entity.Dispute{}, err
	}

	if err := insertEvent(ctx, tx, audit); err != nil {
		return entity.Dispute{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.Dispute{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Dispute{}, err
	}

	return d, nil
}

// GetByTrade returns the latest dispute for a trade (newest first).
func (r *dispute) GetByTrade(ctx context.Context, tradeID string) (entity.Dispute, error) {
	ctx, span := r.tracer.Start(ctx, "dispute.GetByTrade")
	defer span.End()

	query := `SELECT ` + disputeColumns + ` FROM dispute WHERE trade_id = $1 ORDER BY created_at DESC LIMIT 1`

	d, err := scanDispute(r.db.QueryRowContext(ctx, query, tradeID))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Dispute{}, biz.ErrResourceNotFound
	}

	if err != nil {
		r.logger.WarnContext(ctx, "get dispute by trade failed", "error", err)

		return entity.Dispute{}, err
	}

	return d, nil
}

// GetByID returns a dispute by id.
func (r *dispute) GetByID(ctx context.Context, id uuid.UUID) (entity.Dispute, error) {
	ctx, span := r.tracer.Start(ctx, "dispute.GetByID")
	defer span.End()

	query := `SELECT ` + disputeColumns + ` FROM dispute WHERE id = $1`

	d, err := scanDispute(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Dispute{}, biz.ErrResourceNotFound
	}

	if err != nil {
		r.logger.WarnContext(ctx, "get dispute by id failed", "error", err)

		return entity.Dispute{}, err
	}

	return d, nil
}

// ListEvents returns the immutable audit trail oldest-first.
func (r *dispute) ListEvents(ctx context.Context, disputeID uuid.UUID) ([]entity.DisputeEvent, error) {
	ctx, span := r.tracer.Start(ctx, "dispute.ListEvents")
	defer span.End()

	const query = `
		SELECT id, dispute_id, actor_account_id, action, detail_ref, created_at
		FROM dispute_event
		WHERE dispute_id = $1
		ORDER BY created_at, id`

	rows, err := r.db.QueryContext(ctx, query, disputeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []entity.DisputeEvent

	for rows.Next() {
		var e entity.DisputeEvent
		if err := rows.Scan(&e.ID, &e.DisputeID, &e.ActorAccountID, &e.Action, &e.DetailRef, &e.CreatedAt); err != nil {
			r.logger.WarnContext(ctx, "scan dispute_event failed", "error", err)

			continue
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// AppendEvent inserts an immutable audit row (no state change).
func (r *dispute) AppendEvent(ctx context.Context, audit entity.DisputeEvent) error {
	ctx, span := r.tracer.Start(ctx, "dispute.AppendEvent")
	defer span.End()

	return insertEvent(ctx, r.db, audit)
}

// TransitionTx writes the new state (CAS on the from-state) and appends the audit
// row in one transaction. A CAS miss (row absent or not in from-state) is an
// illegal/lost transition -> ErrResourceInvalid.
func (r *dispute) TransitionTx(
	ctx context.Context,
	id uuid.UUID,
	from, to entity.State,
	audit entity.DisputeEvent,
) (entity.Dispute, error) {
	ctx, span := r.tracer.Start(ctx, "dispute.TransitionTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Dispute{}, err
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		UPDATE dispute SET state = $1
		WHERE id = $2 AND state = $3
		RETURNING ` + disputeColumns

	d, err := scanDispute(tx.QueryRowContext(ctx, query, to, id, from))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Dispute{}, biz.ErrResourceInvalid
	}

	if err != nil {
		return entity.Dispute{}, err
	}

	if err := insertEvent(ctx, tx, audit); err != nil {
		return entity.Dispute{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Dispute{}, err
	}

	return d, nil
}

// ResolveTx sets ruling+RESOLVED (CAS on from-state), appends the RULED audit row,
// and writes the dispute.resolved outbox row in one transaction. The CAS on
// from-state makes the ruling immutable: a second resolve finds the row already
// RESOLVED and returns ErrResourceInvalid.
func (r *dispute) ResolveTx(
	ctx context.Context,
	id uuid.UUID,
	from entity.State,
	ruling entity.Ruling,
	ruledBy uuid.UUID,
	audit entity.DisputeEvent,
	outbox entity.OutboxEvent,
) (entity.Dispute, error) {
	ctx, span := r.tracer.Start(ctx, "dispute.ResolveTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Dispute{}, err
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		UPDATE dispute
		SET state = $1, ruling = $2, ruled_by = $3, resolved_at = NOW()
		WHERE id = $4 AND state = $5
		RETURNING ` + disputeColumns

	d, err := scanDispute(tx.QueryRowContext(ctx, query, entity.StateResolved, ruling, ruledBy, id, from))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Dispute{}, biz.ErrResourceInvalid
	}

	if err != nil {
		return entity.Dispute{}, err
	}

	if err := insertEvent(ctx, tx, audit); err != nil {
		return entity.Dispute{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.Dispute{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Dispute{}, err
	}

	return d, nil
}

// ListByState returns disputes newest-first, optionally filtered by state.
func (r *dispute) ListByState(ctx context.Context, state entity.State) ([]entity.Dispute, error) {
	ctx, span := r.tracer.Start(ctx, "dispute.ListByState")
	defer span.End()

	query := `SELECT ` + disputeColumns + ` FROM dispute`
	args := []any{}

	if state != "" {
		query += ` WHERE state = $1`
		args = append(args, state)
	}

	query += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var disputes []entity.Dispute

	for rows.Next() {
		d, err := scanDispute(rows)
		if err != nil {
			r.logger.WarnContext(ctx, "scan dispute failed", "error", err)

			continue
		}

		disputes = append(disputes, d)
	}

	return disputes, rows.Err()
}

// execer is satisfied by both *sql.DB and *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// insertEvent appends an immutable dispute_event row.
func insertEvent(ctx context.Context, db execer, audit entity.DisputeEvent) error {
	const query = `
		INSERT INTO dispute_event (id, dispute_id, actor_account_id, action, detail_ref, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := db.ExecContext(ctx, query,
		audit.ID, audit.DisputeID, audit.ActorAccountID, audit.Action, audit.DetailRef, audit.CreatedAt)

	return err
}

// insertOutbox writes one outbox row (idempotent on idempotency_key).
func insertOutbox(ctx context.Context, db execer, outbox entity.OutboxEvent) error {
	const query = `
		INSERT INTO outbox (id, subject, idempotency_key, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING`

	_, err := db.ExecContext(ctx, query, outbox.ID, outbox.Subject, outbox.IdempotencyKey, outbox.Payload)

	return err
}

// isUniqueViolation reports whether err is a Postgres 23505 unique violation.
// Kept string-based to avoid importing the pgconn type here.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key value")
}
