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
	"strings"

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

const auctionColumns = `id, lot_id, atype, state, closes_at, reserve_cents,
	winner_account_id, cleared_price_cents, created_at`

func scanAuction(row interface{ Scan(...any) error }) (entity.Auction, error) {
	var (
		a            entity.Auction
		atype, state string
		winner       uuid.NullUUID
	)

	if err := row.Scan(
		&a.ID, &a.LotID, &atype, &state, &a.ClosesAt, &a.ReserveCents,
		&winner, &a.ClearedPriceCents, &a.CreatedAt,
	); err != nil {
		return entity.Auction{}, err
	}

	a.Atype = entity.AuctionMode(atype)
	a.State = entity.AuctionState(state)

	if winner.Valid {
		w := winner.UUID
		a.WinnerAccountID = &w
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

// ParticipantCount implements biz.RepositoryAuction: distinct bidders.
func (r *auction) ParticipantCount(ctx context.Context, auctionID uuid.UUID) (int, error) {
	ctx, span := r.tracer.Start(ctx, "auction.ParticipantCount")
	defer span.End()

	const query = `SELECT COUNT(DISTINCT bidder_account_id) FROM passive_bid WHERE auction_id = $1`

	var count int
	if err := r.db.QueryRowContext(ctx, query, auctionID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// BidsByAuction implements biz.RepositoryAuction: the immutable log, placed_at order.
func (r *auction) BidsByAuction(ctx context.Context, auctionID uuid.UUID) ([]entity.PassiveBid, error) {
	ctx, span := r.tracer.Start(ctx, "auction.BidsByAuction")
	defer span.End()

	const query = `
		SELECT id, auction_id, bidder_account_id, price_cents, placed_at, debit_idempotency_key, created_at
		FROM passive_bid
		WHERE auction_id = $1
		ORDER BY placed_at, id`

	return r.queryBids(ctx, query, auctionID)
}

// BidsByBidder implements biz.RepositoryAuction: the caller's own bids.
func (r *auction) BidsByBidder(ctx context.Context, auctionID, bidderID uuid.UUID) ([]entity.PassiveBid, error) {
	ctx, span := r.tracer.Start(ctx, "auction.BidsByBidder")
	defer span.End()

	const query = `
		SELECT id, auction_id, bidder_account_id, price_cents, placed_at, debit_idempotency_key, created_at
		FROM passive_bid
		WHERE auction_id = $1 AND bidder_account_id = $2
		ORDER BY placed_at, id`

	return r.queryBids(ctx, query, auctionID, bidderID)
}

func (r *auction) queryBids(ctx context.Context, query string, args ...any) ([]entity.PassiveBid, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bids []entity.PassiveBid

	for rows.Next() {
		var b entity.PassiveBid
		if err := rows.Scan(
			&b.ID, &b.AuctionID, &b.BidderAccountID, &b.PriceCents,
			&b.PlacedAt, &b.DebitIdempotencyKey, &b.CreatedAt,
		); err != nil {
			r.logger.WarnContext(ctx, "scan bid row failed", "error", err)

			continue
		}

		bids = append(bids, b)
	}

	return bids, rows.Err()
}

// HasBid implements biz.RepositoryAuction.
func (r *auction) HasBid(ctx context.Context, auctionID, bidderID uuid.UUID) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "auction.HasBid")
	defer span.End()

	const query = `SELECT EXISTS (
		SELECT 1 FROM passive_bid WHERE auction_id = $1 AND bidder_account_id = $2)`

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, auctionID, bidderID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

// CreateAuctionTx implements biz.RepositoryAuction: insert an OPEN auction AND
// mark the inbox key consumed in one tx. A duplicate inbox key (replayed event)
// or a lot that already has an auction yields biz.ErrResourceExists so the use
// case can treat it as an idempotent no-op.
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
			id, lot_id, atype, state, closes_at, reserve_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (lot_id) DO NOTHING`

	res, err = tx.ExecContext(ctx, insertQuery,
		a.ID, a.LotID, a.Atype, a.State, a.ClosesAt, a.ReserveCents, a.CreatedAt)
	if err != nil {
		return err
	}

	if affected, _ := res.RowsAffected(); affected == 0 {
		return biz.ErrResourceExists
	}

	return tx.Commit()
}

// InsertBidTx implements biz.RepositoryAuction: insert an immutable bid row and
// write the bid.placed outbox event in one tx, conditional on the auction still
// being OPEN. A duplicate (auction,bidder,price) or replayed debit key yields
// biz.ErrResourceExists; a non-OPEN auction yields biz.ErrResourceInvalid.
func (r *auction) InsertBidTx(ctx context.Context, b entity.PassiveBid, vickreyOneBid bool, outbox entity.OutboxEvent) error {
	ctx, span := r.tracer.Start(ctx, "auction.InsertBidTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Guard: the auction must still be OPEN at insert time (it may have closed
	// between the use case's read and now).
	const stateQuery = `SELECT state FROM auction WHERE id = $1`

	var state string
	if err := tx.QueryRowContext(ctx, stateQuery, b.AuctionID).Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return biz.ErrResourceNotFound
		}

		return err
	}

	if entity.AuctionState(state) != entity.StateOpen {
		return fmt.Errorf("%w: auction is %s, not OPEN", biz.ErrResourceInvalid, state)
	}

	// VICKREY one-bid guard inside the tx (the unique index is the backstop).
	if vickreyOneBid {
		const existsQuery = `SELECT EXISTS (
			SELECT 1 FROM passive_bid WHERE auction_id = $1 AND bidder_account_id = $2)`

		var exists bool
		if err := tx.QueryRowContext(ctx, existsQuery, b.AuctionID, b.BidderAccountID).Scan(&exists); err != nil {
			return err
		}

		if exists {
			return fmt.Errorf("%w: VICKREY allows one sealed bid per bidder", biz.ErrResourceExists)
		}
	}

	const insertQuery = `
		INSERT INTO passive_bid (
			id, auction_id, bidder_account_id, price_cents, placed_at, debit_idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if _, err := tx.ExecContext(ctx, insertQuery,
		b.ID, b.AuctionID, b.BidderAccountID, b.PriceCents, b.PlacedAt, b.DebitIdempotencyKey, b.CreatedAt); err != nil {
		// Unique violation (duplicate price for bidder, or replayed debit key).
		if isUniqueViolation(err) {
			return biz.ErrResourceExists
		}

		return err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return err
	}

	return tx.Commit()
}

// CloseTx implements biz.RepositoryAuction: conditionally flip OPEN -> CLOSING
// and write the auction.closed outbox event in one tx. A 0-row update means the
// auction is no longer OPEN -> ErrResourceInvalid.
func (r *auction) CloseTx(ctx context.Context, auctionID uuid.UUID, closedOutbox entity.OutboxEvent) (entity.Auction, error) {
	ctx, span := r.tracer.Start(ctx, "auction.CloseTx")
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

	a, err := scanAuction(tx.QueryRowContext(ctx, updateQuery, entity.StateClosing, auctionID, entity.StateOpen))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Auction{}, fmt.Errorf("%w: auction not OPEN", biz.ErrResourceInvalid)
	}

	if err != nil {
		return entity.Auction{}, err
	}

	if err := insertOutbox(ctx, tx, closedOutbox); err != nil {
		return entity.Auction{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Auction{}, err
	}

	return a, nil
}

// ResolveTx implements biz.RepositoryAuction: conditionally flip CLOSING ->
// RESOLVED (winner) or CLOSING -> ABORTED (no winner), record winner + cleared
// price, and write the auction.won outbox event when a winner exists, all in one
// tx. A 0-row update -> ErrResourceInvalid.
func (r *auction) ResolveTx(ctx context.Context, auctionID uuid.UUID, res biz.Result, wonOutbox *entity.OutboxEvent) (entity.Auction, error) {
	ctx, span := r.tracer.Start(ctx, "auction.ResolveTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Auction{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		newState entity.AuctionState
		winner   uuid.NullUUID
		cleared  int64
	)

	if res.Won {
		newState = entity.StateResolved
		winner = uuid.NullUUID{UUID: res.WinnerAccountID, Valid: true}
		cleared = res.ClearedPriceCents
	} else {
		newState = entity.StateAborted
	}

	updateQuery := `
		UPDATE auction SET state = $1, winner_account_id = $2, cleared_price_cents = $3
		WHERE id = $4 AND state = $5
		RETURNING ` + auctionColumns

	a, err := scanAuction(tx.QueryRowContext(ctx, updateQuery,
		newState, winner, cleared, auctionID, entity.StateClosing))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Auction{}, fmt.Errorf("%w: auction not CLOSING", biz.ErrResourceInvalid)
	}

	if err != nil {
		return entity.Auction{}, err
	}

	if wonOutbox != nil {
		if err := insertOutbox(ctx, tx, *wonOutbox); err != nil {
			return entity.Auction{}, err
		}
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

// isUniqueViolation reports whether err is a Postgres unique-constraint violation
// (SQLSTATE 23505), used to map a duplicate bid to ErrResourceExists without a
// hard pgx dependency in this thin helper.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
