package repo

import (
	"application/internal/biz"
	"application/internal/datasource"
	"application/internal/entity"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type lot struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryLot = (*lot)(nil)

// NewLot constructs the lot repository (raw parameterized pgx SQL with an OTel
// tracer, like the template's placeholder repo).
func NewLot(logger *slog.Logger, db *datasource.PostgresDB) *lot {
	return &lot{
		logger: logger.With("layer", "LotRepo"),
		tracer: otel.Tracer("LotRepo"),
		db:     db,
	}
}

const lotColumns = `id, object_id, seller_account_id, title, description, atype,
	duration_days, reserve_cents, appraised_value_cents, state, iso_week,
	created_at, scheduled_at,
	COALESCE(category_code, ''), COALESCE(certified, FALSE), inspector_account_id,
	COALESCE(authenticity, ''), COALESCE(condition_grade, '')`

// scanLot scans a single lot row in lotColumns order.
func scanLot(row interface{ Scan(...any) error }) (entity.Lot, error) {
	var (
		l           entity.Lot
		mode, state string
		duration    sql.NullInt32
		scheduledAt sql.NullTime
		inspector   uuid.NullUUID
	)

	if err := row.Scan(
		&l.ID, &l.ObjectID, &l.SellerAccountID, &l.Title, &l.Description, &mode,
		&duration, &l.ReserveCents, &l.AppraisedValueCents, &state, &l.ISOWeek,
		&l.CreatedAt, &scheduledAt,
		&l.CategoryCode, &l.Certified, &inspector, &l.Authenticity, &l.ConditionGrade,
	); err != nil {
		return entity.Lot{}, err
	}

	l.Mode = entity.AuctionMode(mode)
	l.State = entity.LotState(state)

	if duration.Valid {
		d := duration.Int32
		l.DurationDays = &d
	}

	if scheduledAt.Valid {
		t := scheduledAt.Time
		l.ScheduledAt = &t
	}

	if inspector.Valid {
		id := inspector.UUID
		l.InspectorID = &id
	}

	return l, nil
}

// GetWeekly implements biz.RepositoryLot: SCHEDULED lots for an ISO week.
func (r *lot) GetWeekly(ctx context.Context, week string) ([]entity.Lot, error) {
	ctx, span := r.tracer.Start(ctx, "lot.GetWeekly")
	defer span.End()

	query := `SELECT ` + lotColumns + `
		FROM lot
		WHERE iso_week = $1 AND state = $2
		ORDER BY scheduled_at, created_at`

	rows, err := r.db.QueryContext(ctx, query, week, entity.LotScheduled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.collect(rows)
}

// Get implements biz.RepositoryLot.
func (r *lot) Get(ctx context.Context, id uuid.UUID) (entity.Lot, error) {
	ctx, span := r.tracer.Start(ctx, "lot.Get")
	defer span.End()

	query := `SELECT ` + lotColumns + ` FROM lot WHERE id = $1`

	l, err := scanLot(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Lot{}, biz.ErrResourceNotFound
	}

	if err != nil {
		r.logger.WarnContext(ctx, "get lot failed", "error", err)

		return entity.Lot{}, err
	}

	return l, nil
}

// List implements biz.RepositoryLot: optional state/week filters.
func (r *lot) List(ctx context.Context, filter biz.LotListFilter) ([]entity.Lot, error) {
	ctx, span := r.tracer.Start(ctx, "lot.List")
	defer span.End()

	query := `SELECT ` + lotColumns + ` FROM lot WHERE 1 = 1`
	args := make([]any, 0, 2)

	if filter.State != "" {
		args = append(args, filter.State)
		query += fmt.Sprintf(" AND state = $%d", len(args))
	}

	if filter.ISOWeek != "" {
		args = append(args, filter.ISOWeek)
		query += fmt.Sprintf(" AND iso_week = $%d", len(args))
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.collect(rows)
}

func (r *lot) collect(rows *sql.Rows) ([]entity.Lot, error) {
	var lots []entity.Lot

	for rows.Next() {
		l, err := scanLot(rows)
		if err != nil {
			r.logger.Warn("scan lot row failed", "error", err)

			continue
		}

		lots = append(lots, l)
	}

	return lots, rows.Err()
}

// AttestationsByLot implements biz.RepositoryLot.
func (r *lot) AttestationsByLot(ctx context.Context, lotID uuid.UUID) ([]entity.Attestation, error) {
	ctx, span := r.tracer.Start(ctx, "lot.AttestationsByLot")
	defer span.End()

	const query = `
		SELECT id, lot_id, inspector_id, result, notes_ref, recorded_at
		FROM attestation
		WHERE lot_id = $1
		ORDER BY recorded_at`

	rows, err := r.db.QueryContext(ctx, query, lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atts []entity.Attestation

	for rows.Next() {
		var (
			a      entity.Attestation
			result string
		)

		if err := rows.Scan(&a.ID, &a.LotID, &a.InspectorID, &result, &a.NotesRef, &a.RecordedAt); err != nil {
			r.logger.WarnContext(ctx, "scan attestation row failed", "error", err)

			continue
		}

		a.Result = entity.AttestationResult(result)
		atts = append(atts, a)
	}

	return atts, rows.Err()
}

// HasPassAttestation implements biz.RepositoryLot.
func (r *lot) HasPassAttestation(ctx context.Context, lotID uuid.UUID) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "lot.HasPassAttestation")
	defer span.End()

	const query = `SELECT EXISTS (
		SELECT 1 FROM attestation WHERE lot_id = $1 AND result = 'PASS')`

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, lotID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

// CreateLotTx implements biz.RepositoryLot: insert a DRAFT lot AND mark the inbox
// key consumed in one transaction. A duplicate inbox key (replayed event) or an
// object that already has a lot yields biz.ErrResourceExists so the use case can
// treat it as an idempotent no-op.
func (r *lot) CreateLotTx(ctx context.Context, l entity.Lot, inboxKey string) error {
	ctx, span := r.tracer.Start(ctx, "lot.CreateLotTx")
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

	var duration any
	if l.DurationDays != nil {
		duration = *l.DurationDays
	}

	const insertQuery = `
		INSERT INTO lot (
			id, object_id, seller_account_id, title, description, atype,
			duration_days, reserve_cents, appraised_value_cents, state, iso_week, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (object_id) DO NOTHING`

	res, err = tx.ExecContext(ctx, insertQuery,
		l.ID, l.ObjectID, l.SellerAccountID, l.Title, l.Description, l.Mode,
		duration, l.ReserveCents, l.AppraisedValueCents, l.State, l.ISOWeek, l.CreatedAt)
	if err != nil {
		return err
	}

	// The object already has a lot (out-of-band duplicate): treat as a no-op so
	// the inbox + uniqueness both guard idempotency.
	if affected, _ := res.RowsAffected(); affected == 0 {
		return biz.ErrResourceExists
	}

	return tx.Commit()
}

// RecordAttestationTx implements biz.RepositoryLot: insert the attestation,
// optionally flip the lot to REJECTED, and write the outbox event, all in one tx.
func (r *lot) RecordAttestationTx(
	ctx context.Context,
	att entity.Attestation,
	rejectLot bool,
	outbox entity.OutboxEvent,
) error {
	ctx, span := r.tracer.Start(ctx, "lot.RecordAttestationTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const insertQuery = `
		INSERT INTO attestation (id, lot_id, inspector_id, result, notes_ref, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	if _, err := tx.ExecContext(ctx, insertQuery,
		att.ID, att.LotID, att.InspectorID, att.Result, att.NotesRef, att.RecordedAt); err != nil {
		return err
	}

	if rejectLot {
		const rejectQuery = `
			UPDATE lot SET state = $1
			WHERE id = $2 AND state = $3`

		if _, err := tx.ExecContext(ctx, rejectQuery, entity.LotRejected, att.LotID, entity.LotDraft); err != nil {
			return err
		}
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return err
	}

	return tx.Commit()
}

// CertifyTx implements biz.RepositoryLot: conditionally flip DRAFT -> CERTIFIED
// and write the outbox event in one tx. A 0-row update means the lot is no longer
// DRAFT -> ErrResourceInvalid.
func (r *lot) CertifyTx(ctx context.Context, lotID uuid.UUID, outbox entity.OutboxEvent) (entity.Lot, error) {
	ctx, span := r.tracer.Start(ctx, "lot.CertifyTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Lot{}, err
	}
	defer func() { _ = tx.Rollback() }()

	updateQuery := `
		UPDATE lot SET state = $1
		WHERE id = $2 AND state = $3
		RETURNING ` + lotColumns

	l, err := scanLot(tx.QueryRowContext(ctx, updateQuery, entity.LotCertified, lotID, entity.LotDraft))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Lot{}, fmt.Errorf("%w: lot not DRAFT", biz.ErrResourceInvalid)
	}

	if err != nil {
		return entity.Lot{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.Lot{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Lot{}, err
	}

	return l, nil
}

// ScheduleTx implements biz.RepositoryLot: conditionally flip CERTIFIED ->
// SCHEDULED and write the outbox event in one tx. The conditional update is gated
// on BOTH (a) the row still being CERTIFIED and (b) fewer than `cap` lots already
// SCHEDULED in the lot's ISO week — enforcing the weekly 32-cap atomically. A
// 0-row update (cap reached or state changed) -> ErrResourceInvalid.
func (r *lot) ScheduleTx(
	ctx context.Context,
	lotID uuid.UUID,
	scheduledAt time.Time,
	weekCap int,
	outbox entity.OutboxEvent,
) (entity.Lot, error) {
	ctx, span := r.tracer.Start(ctx, "lot.ScheduleTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Lot{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// The cap subquery counts already-SCHEDULED lots for this lot's week; the
	// UPDATE only fires when that count is below the cap AND the row is CERTIFIED.
	updateQuery := `
		UPDATE lot SET state = $1, scheduled_at = $2
		WHERE id = $3
		  AND state = $4
		  AND (
			SELECT COUNT(*) FROM lot s
			WHERE s.iso_week = lot.iso_week AND s.state = $1
		  ) < $5
		RETURNING ` + lotColumns

	l, err := scanLot(tx.QueryRowContext(ctx, updateQuery,
		entity.LotScheduled, scheduledAt, lotID, entity.LotCertified, weekCap))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Lot{}, fmt.Errorf("%w: lot not CERTIFIED or weekly cap reached", biz.ErrResourceInvalid)
	}

	if err != nil {
		return entity.Lot{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.Lot{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Lot{}, err
	}

	return l, nil
}

// InspectTx records an Inspector's sealing verdict in one transaction: insert the
// inspection row (one per lot), seal the lot's columns, transition DRAFT ->
// CERTIFIED (approve) or DRAFT -> REJECTED (reject), and write the outbox events.
// The conditional lot UPDATE is gated on the row still being DRAFT, so a 0-row
// update -> ErrResourceInvalid (already inspected / not awaiting inspection).
func (r *lot) InspectTx(
	ctx context.Context,
	insp entity.Inspection,
	approve bool,
	outboxes []entity.OutboxEvent,
) (entity.Lot, error) {
	ctx, span := r.tracer.Start(ctx, "lot.InspectTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Lot{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// One verdict per lot. A conflict means the lot was already inspected.
	const inspectQuery = `
		INSERT INTO inspection (id, lot_id, inspector_id, verdict, authenticity, condition_grade, notes, sealed_at)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8)
		ON CONFLICT (lot_id) DO NOTHING`

	res, err := tx.ExecContext(ctx, inspectQuery,
		insp.ID, insp.LotID, insp.InspectorID, insp.Verdict,
		insp.Authenticity, insp.ConditionGrade, insp.Notes, insp.SealedAt)
	if err != nil {
		return entity.Lot{}, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return entity.Lot{}, fmt.Errorf("%w: lot already inspected", biz.ErrResourceInvalid)
	}

	newState := entity.LotRejected
	if approve {
		newState = entity.LotCertified
	}

	const updateQuery = `
		UPDATE lot SET
			state = $1, certified = $2, inspector_account_id = $3,
			authenticity = $4, condition_grade = NULLIF($5, '')
		WHERE id = $6 AND state = $7
		RETURNING ` + lotColumns

	l, err := scanLot(tx.QueryRowContext(ctx, updateQuery,
		newState, approve, insp.InspectorID, insp.Authenticity, insp.ConditionGrade,
		insp.LotID, entity.LotDraft))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Lot{}, fmt.Errorf("%w: lot not awaiting inspection (not DRAFT)", biz.ErrResourceInvalid)
	}
	if err != nil {
		return entity.Lot{}, err
	}

	for _, o := range outboxes {
		if err := insertOutbox(ctx, tx, o); err != nil {
			return entity.Lot{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return entity.Lot{}, err
	}

	return l, nil
}

// insertOutbox appends the outbox event row within the given transaction.
func insertOutbox(ctx context.Context, tx *sql.Tx, outbox entity.OutboxEvent) error {
	const outboxQuery = `
		INSERT INTO outbox (id, subject, idempotency_key, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING`

	_, err := tx.ExecContext(ctx, outboxQuery,
		outbox.ID, outbox.Subject, outbox.IdempotencyKey, outbox.Payload)

	return err
}
