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

type auction struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryAuction = (*auction)(nil)

// NewAuction constructs the auction repository (raw parameterized pgx SQL with an
// OTel tracer, like the template's placeholder repo).
func NewAuction(logger *slog.Logger, db *datasource.PostgresDB) *auction {
	return &auction{
		logger: logger.With("layer", "AuctionRepo"),
		tracer: otel.Tracer("AuctionRepo"),
		db:     db,
	}
}

const auctionColumns = `id, lot_id, state, ceiling_cents, floor_cents,
	drop_step_cents, drop_interval_seconds, open_at, hammer_at,
	winner_account_id, hammer_price_cents, created_at`

// scanAuction scans a single auction row in auctionColumns order.
func scanAuction(row interface{ Scan(...any) error }) (entity.Auction, error) {
	var (
		a          entity.Auction
		state      string
		openAt     sql.NullTime
		hammerAt   sql.NullTime
		winner     sql.NullString
		hammerCent sql.NullInt64
	)

	if err := row.Scan(
		&a.ID, &a.LotID, &state, &a.CeilingCents, &a.FloorCents,
		&a.DropStepCents, &a.DropIntervalSeconds, &openAt, &hammerAt,
		&winner, &hammerCent, &a.CreatedAt,
	); err != nil {
		return entity.Auction{}, err
	}

	a.State = entity.AuctionState(state)

	if openAt.Valid {
		t := openAt.Time
		a.OpenAt = &t
	}

	if hammerAt.Valid {
		t := hammerAt.Time
		a.HammerAt = &t
	}

	if winner.Valid {
		if id, err := uuid.Parse(winner.String); err == nil {
			a.WinnerAccountID = &id
		}
	}

	if hammerCent.Valid {
		c := hammerCent.Int64
		a.HammerPriceCents = &c
	}

	return a, nil
}

// Get implements biz.RepositoryAuction.
func (r *auction) Get(ctx context.Context, id uuid.UUID) (entity.Auction, error) {
	ctx, span := r.tracer.Start(ctx, "auction.Get")
	defer span.End()

	query := `SELECT ` + auctionColumns + ` FROM auction WHERE id = $1`

	a, err := scanAuction(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Auction{}, biz.ErrResourceNotFound
	}

	if err != nil {
		r.logger.WarnContext(ctx, "get auction failed", "error", err)

		return entity.Auction{}, err
	}

	return a, nil
}

// GetParticipant implements biz.RepositoryAuction.
func (r *auction) GetParticipant(ctx context.Context, auctionID, accountID uuid.UUID) (entity.Participant, error) {
	ctx, span := r.tracer.Start(ctx, "auction.GetParticipant")
	defer span.End()

	const query = `
		SELECT auction_id, account_id, kyc_approved, tier, reservation_state, full_lock_state, joined_at
		FROM auction_participant
		WHERE auction_id = $1 AND account_id = $2`

	var (
		p        entity.Participant
		tier     string
		resState string
		fullLock string
	)

	err := r.db.QueryRowContext(ctx, query, auctionID, accountID).
		Scan(&p.AuctionID, &p.AccountID, &p.KycApproved, &tier, &resState, &fullLock, &p.JoinedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Participant{}, biz.ErrResourceNotFound
	}

	if err != nil {
		return entity.Participant{}, err
	}

	p.Tier = entity.Tier(tier)
	p.ReservationSt = entity.ReservationState(resState)
	p.FullLockState = entity.ReservationState(fullLock)

	return p, nil
}

// CountEligibleParticipants implements biz.RepositoryAuction: how many
// participants satisfy the full entry-to-OPEN gate (root CLAUDE.md §3).
func (r *auction) CountEligibleParticipants(ctx context.Context, auctionID uuid.UUID) (int, error) {
	ctx, span := r.tracer.Start(ctx, "auction.CountEligibleParticipants")
	defer span.End()

	const query = `
		SELECT COUNT(*)
		FROM auction_participant
		WHERE auction_id = $1
		  AND kyc_approved = TRUE
		  AND tier IN ('MEMBER', 'VIP')
		  AND reservation_state = 'LOCKED'
		  AND full_lock_state = 'LOCKED'`

	var n int
	if err := r.db.QueryRowContext(ctx, query, auctionID).Scan(&n); err != nil {
		return 0, err
	}

	return n, nil
}

// CreateAuctionTx implements biz.RepositoryAuction: insert a SCHEDULED auction AND
// mark the inbox key consumed in one tx. A duplicate inbox key (replayed event)
// or an existing auction id yields biz.ErrResourceExists so the use case no-ops.
func (r *auction) CreateAuctionTx(ctx context.Context, a entity.Auction, inboxKey string) error {
	ctx, span := r.tracer.Start(ctx, "auction.CreateAuctionTx")
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

	const insertQuery = `
		INSERT INTO auction (
			id, lot_id, state, ceiling_cents, floor_cents,
			drop_step_cents, drop_interval_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING`

	res, err = tx.ExecContext(ctx, insertQuery,
		a.ID, a.LotID, a.State, a.CeilingCents, a.FloorCents,
		a.DropStepCents, a.DropIntervalSeconds, a.CreatedAt)
	if err != nil {
		return err
	}

	if affected, _ := res.RowsAffected(); affected == 0 {
		return biz.ErrResourceExists
	}

	return tx.Commit()
}

// ReserveTx implements biz.RepositoryAuction: upsert the participant, insert the
// REQUESTED reservation, and write the outbox event in one tx. Idempotent on the
// reservation's escrow_ref (ON CONFLICT DO NOTHING); a duplicate returns the
// existing row without re-emitting.
func (r *auction) ReserveTx(
	ctx context.Context,
	p entity.Participant,
	res entity.Reservation,
	outbox entity.OutboxEvent,
) (entity.Reservation, error) {
	ctx, span := r.tracer.Start(ctx, "auction.ReserveTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Reservation{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Upsert the participant: create on first sight, refresh cached eligibility on
	// subsequent locks. Lock states are only ever advanced by escrow.locked, so we
	// don't clobber them here (COALESCE keeps the stronger existing value).
	const upsertParticipant = `
		INSERT INTO auction_participant (
			auction_id, account_id, kyc_approved, tier,
			reservation_state, full_lock_state, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (auction_id, account_id) DO UPDATE
		SET kyc_approved = EXCLUDED.kyc_approved,
		    tier = EXCLUDED.tier`

	if _, err := tx.ExecContext(ctx, upsertParticipant,
		p.AuctionID, p.AccountID, p.KycApproved, p.Tier,
		entity.ReservationRequested, entity.ReservationRequested, p.JoinedAt); err != nil {
		return entity.Reservation{}, err
	}

	// Insert the reservation; dedup on escrow_ref so a retried reserve/lock is a
	// no-op (and we then return the existing row).
	const insertReservation = `
		INSERT INTO reservation (
			id, auction_id, account_id, kind, amount_cents, state, escrow_ref, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (escrow_ref) DO NOTHING`

	insRes, err := tx.ExecContext(ctx, insertReservation,
		res.ID, res.AuctionID, res.AccountID, res.Kind, res.AmountCents,
		res.State, res.EscrowRef, res.CreatedAt)
	if err != nil {
		return entity.Reservation{}, err
	}

	fresh := true
	if affected, _ := insRes.RowsAffected(); affected == 0 {
		fresh = false
	}

	// Only emit the lock request for a fresh reservation (outbox dedups on
	// idempotency_key too, but skipping keeps the relay lean).
	if fresh {
		if err := insertOutbox(ctx, tx, outbox); err != nil {
			return entity.Reservation{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return entity.Reservation{}, err
	}

	// Return the canonical stored row (handles the duplicate case).
	return r.reservationByRef(ctx, res.EscrowRef)
}

// reservationByRef loads a reservation by its escrow_ref.
func (r *auction) reservationByRef(ctx context.Context, escrowRef string) (entity.Reservation, error) {
	const query = `
		SELECT id, auction_id, account_id, kind, amount_cents, state, escrow_ref, created_at
		FROM reservation WHERE escrow_ref = $1`

	var (
		res   entity.Reservation
		kind  string
		state string
	)

	err := r.db.QueryRowContext(ctx, query, escrowRef).
		Scan(&res.ID, &res.AuctionID, &res.AccountID, &kind, &res.AmountCents, &state, &res.EscrowRef, &res.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Reservation{}, biz.ErrResourceNotFound
	}

	if err != nil {
		return entity.Reservation{}, err
	}

	res.Kind = entity.ReservationKind(kind)
	res.State = entity.ReservationState(state)

	return res, nil
}

// ApplyEscrowLockedTx implements biz.RepositoryAuction: flip the reservation
// identified by escrowRef from REQUESTED to LOCKED, advance the participant's
// matching lock flag, and mark the inbox key consumed, all in one tx. Returns
// ErrResourceExists on a duplicate inbox key and ErrResourceNotFound when no
// reservation matches escrowRef.
func (r *auction) ApplyEscrowLockedTx(ctx context.Context, escrowRef, inboxKey string) error {
	ctx, span := r.tracer.Start(ctx, "auction.ApplyEscrowLockedTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const inboxQuery = `
		INSERT INTO consumed_event (idempotency_key) VALUES ($1)
		ON CONFLICT (idempotency_key) DO NOTHING`

	inboxRes, err := tx.ExecContext(ctx, inboxQuery, inboxKey)
	if err != nil {
		return err
	}

	if affected, _ := inboxRes.RowsAffected(); affected == 0 {
		return biz.ErrResourceExists
	}

	// Flip the reservation to LOCKED, returning the kind + identity so we can update
	// the right participant flag. A 0-row update means the ref is unknown.
	const lockReservation = `
		UPDATE reservation SET state = 'LOCKED'
		WHERE escrow_ref = $1 AND state = 'REQUESTED'
		RETURNING auction_id, account_id, kind`

	var (
		auctionID uuid.UUID
		accountID uuid.UUID
		kind      string
	)

	err = tx.QueryRowContext(ctx, lockReservation, escrowRef).Scan(&auctionID, &accountID, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return biz.ErrResourceNotFound
	}

	if err != nil {
		return err
	}

	// Advance the matching participant lock flag.
	column := "reservation_state"
	if entity.ReservationKind(kind) == entity.KindFullLock {
		column = "full_lock_state"
	}

	updateParticipant := fmt.Sprintf(`
		UPDATE auction_participant SET %s = 'LOCKED'
		WHERE auction_id = $1 AND account_id = $2`, column)

	if _, err := tx.ExecContext(ctx, updateParticipant, auctionID, accountID); err != nil {
		return err
	}

	return tx.Commit()
}

// HammerTx implements biz.RepositoryAuction: conditionally transition OPEN ->
// HAMMER recording winner + price + the outbox event, in one tx. A 0-row update
// (already hammered / not OPEN) -> ErrResourceInvalid. First valid buy wins.
func (r *auction) HammerTx(
	ctx context.Context,
	auctionID, winner uuid.UUID,
	priceCents int64,
	hammerAt time.Time,
	outbox entity.OutboxEvent,
) (entity.Auction, error) {
	ctx, span := r.tracer.Start(ctx, "auction.HammerTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Auction{}, err
	}
	defer func() { _ = tx.Rollback() }()

	updateQuery := `
		UPDATE auction
		SET state = $1, winner_account_id = $2, hammer_price_cents = $3, hammer_at = $4
		WHERE id = $5 AND state = $6
		RETURNING ` + auctionColumns

	a, err := scanAuction(tx.QueryRowContext(ctx, updateQuery,
		entity.AuctionHammer, winner, priceCents, hammerAt, auctionID, entity.AuctionOpen))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Auction{}, fmt.Errorf("%w: auction not OPEN (already hammered?)", biz.ErrResourceInvalid)
	}

	if err != nil {
		return entity.Auction{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.Auction{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Auction{}, err
	}

	return a, nil
}

// OpenTx implements biz.RepositoryAuction: conditionally transition SCHEDULED ->
// OPEN setting open_at + the outbox event, in one tx. 0-row -> ErrResourceInvalid.
func (r *auction) OpenTx(
	ctx context.Context,
	auctionID uuid.UUID,
	openAt time.Time,
	outbox entity.OutboxEvent,
) (entity.Auction, error) {
	ctx, span := r.tracer.Start(ctx, "auction.OpenTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Auction{}, err
	}
	defer func() { _ = tx.Rollback() }()

	updateQuery := `
		UPDATE auction SET state = $1, open_at = $2
		WHERE id = $3 AND state = $4
		RETURNING ` + auctionColumns

	a, err := scanAuction(tx.QueryRowContext(ctx, updateQuery,
		entity.AuctionOpen, openAt, auctionID, entity.AuctionScheduled))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Auction{}, fmt.Errorf("%w: auction not SCHEDULED", biz.ErrResourceInvalid)
	}

	if err != nil {
		return entity.Auction{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.Auction{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Auction{}, err
	}

	return a, nil
}

// TransitionTx implements biz.RepositoryAuction: conditionally flip from->to + the
// outbox event, in one tx. 0-row -> ErrResourceInvalid.
func (r *auction) TransitionTx(
	ctx context.Context,
	auctionID uuid.UUID,
	from, to entity.AuctionState,
	outbox entity.OutboxEvent,
) (entity.Auction, error) {
	ctx, span := r.tracer.Start(ctx, "auction.TransitionTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Auction{}, err
	}
	defer func() { _ = tx.Rollback() }()

	updateQuery := `
		UPDATE auction SET state = $1
		WHERE id = $2 AND state = $3
		RETURNING ` + auctionColumns

	a, err := scanAuction(tx.QueryRowContext(ctx, updateQuery, to, auctionID, from))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Auction{}, fmt.Errorf("%w: auction not in %s", biz.ErrResourceInvalid, from)
	}

	if err != nil {
		return entity.Auction{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.Auction{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Auction{}, err
	}

	return a, nil
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
