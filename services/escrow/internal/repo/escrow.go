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

type escrowRepo struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryEscrow = (*escrowRepo)(nil)

// NewEscrow constructs the escrow repository (raw parameterized pgx SQL with an
// OTel tracer, like the template's placeholder repo). `escrow` is the SOLE writer
// of escrow rows; every mutation is a single transaction.
func NewEscrow(logger *slog.Logger, db *datasource.PostgresDB) *escrowRepo {
	return &escrowRepo{
		logger: logger.With("layer", "EscrowRepo"),
		tracer: otel.Tracer("EscrowRepo"),
		db:     db,
	}
}

const tradeColumns = `id, lot_id, buyer_account_id, seller_account_id, kind, state,
	price_cents, premium_cents, fee_cents, inspector_fee_cents, release_mode,
	funding_deadline, created_at, updated_at`

func scanTrade(s interface{ Scan(...any) error }) (entity.EscrowTrade, error) {
	var (
		t           entity.EscrowTrade
		releaseMode sql.NullString
		deadline    sql.NullTime
	)

	if err := s.Scan(
		&t.ID, &t.LotID, &t.BuyerAccountID, &t.SellerAccountID, &t.Kind, &t.State,
		&t.PriceCents, &t.PremiumCents, &t.FeeCents, &t.InspectorFeeCents, &releaseMode,
		&deadline, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return entity.EscrowTrade{}, err
	}

	if releaseMode.Valid {
		t.ReleaseMode = entity.ReleaseMode(releaseMode.String)
	}

	if deadline.Valid {
		d := deadline.Time
		t.FundingDeadline = &d
	}

	return t, nil
}

// GetTrade implements biz.RepositoryEscrow.
func (r *escrowRepo) GetTrade(ctx context.Context, tradeID uuid.UUID) (entity.EscrowTrade, error) {
	ctx, span := r.tracer.Start(ctx, "escrow.GetTrade")
	defer span.End()

	const query = `SELECT ` + tradeColumns + ` FROM escrow_trade WHERE id = $1`

	t, err := scanTrade(r.db.QueryRowContext(ctx, query, tradeID))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.EscrowTrade{}, biz.ErrResourceNotFound
	}

	if err != nil {
		r.logger.WarnContext(ctx, "get trade failed", "error", err)

		return entity.EscrowTrade{}, err
	}

	return t, nil
}

// ListEntries implements biz.RepositoryEscrow.
func (r *escrowRepo) ListEntries(ctx context.Context, tradeID uuid.UUID) ([]entity.LedgerEntry, error) {
	ctx, span := r.tracer.Start(ctx, "escrow.ListEntries")
	defer span.End()

	const query = `
		SELECT id, trade_id, participant_account_id, entry_type, amount_cents, ref, created_at
		FROM escrow_ledger WHERE trade_id = $1 ORDER BY created_at, id`

	rows, err := r.db.QueryContext(ctx, query, tradeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []entity.LedgerEntry

	for rows.Next() {
		var e entity.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TradeID, &e.ParticipantAccountID, &e.EntryType, &e.AmountCents, &e.Ref, &e.CreatedAt); err != nil {
			r.logger.WarnContext(ctx, "scan ledger row failed", "error", err)

			continue
		}

		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// Balances implements biz.RepositoryEscrow: per-participant SUM(amount_cents).
func (r *escrowRepo) Balances(ctx context.Context, tradeID uuid.UUID) ([]entity.ParticipantBalance, error) {
	ctx, span := r.tracer.Start(ctx, "escrow.Balances")
	defer span.End()

	const query = `
		SELECT participant_account_id, COALESCE(SUM(amount_cents), 0)
		FROM escrow_ledger WHERE trade_id = $1
		GROUP BY participant_account_id`

	rows, err := r.db.QueryContext(ctx, query, tradeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []entity.ParticipantBalance

	for rows.Next() {
		var b entity.ParticipantBalance
		if err := rows.Scan(&b.ParticipantAccountID, &b.BalanceCents); err != nil {
			r.logger.WarnContext(ctx, "scan balance row failed", "error", err)

			continue
		}

		balances = append(balances, b)
	}

	return balances, rows.Err()
}

// CreateTradeTx implements biz.RepositoryEscrow: insert the trade head + initial
// ledger rows + optional outbox row in one tx. Duplicate id -> ErrResourceExists.
func (r *escrowRepo) CreateTradeTx(
	ctx context.Context,
	trade entity.EscrowTrade,
	entries []entity.LedgerEntry,
	outbox *entity.OutboxEvent,
) error {
	ctx, span := r.tracer.Start(ctx, "escrow.CreateTradeTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const insertTrade = `
		INSERT INTO escrow_trade
			(id, lot_id, buyer_account_id, seller_account_id, kind, state,
			 price_cents, premium_cents, fee_cents, inspector_fee_cents, release_mode, funding_deadline)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO NOTHING`

	releaseMode := nullableReleaseMode(trade.ReleaseMode)

	res, err := tx.ExecContext(ctx, insertTrade,
		trade.ID, trade.LotID, trade.BuyerAccountID, trade.SellerAccountID, trade.Kind, trade.State,
		trade.PriceCents, trade.PremiumCents, trade.FeeCents, trade.InspectorFeeCents, releaseMode, trade.FundingDeadline)
	if err != nil {
		return err
	}

	if affected, _ := res.RowsAffected(); affected == 0 {
		return biz.ErrResourceExists
	}

	if err := insertEntries(ctx, tx, entries); err != nil {
		return err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return err
	}

	return tx.Commit()
}

// TransitionTx implements biz.RepositoryEscrow: conditional UPDATE (state = from)
// + ledger rows + optional release_mode + optional outbox, in one tx. Not in
// `from` -> ErrResourceInvalid.
func (r *escrowRepo) TransitionTx(
	ctx context.Context,
	tradeID uuid.UUID,
	from, to entity.EscrowState,
	upd biz.TradeUpdate,
	entries []entity.LedgerEntry,
	outbox *entity.OutboxEvent,
) (entity.EscrowTrade, error) {
	ctx, span := r.tracer.Start(ctx, "escrow.TransitionTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.EscrowTrade{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const updateQuery = `
		UPDATE escrow_trade
		SET state = $1,
		    release_mode = COALESCE($2, release_mode),
		    updated_at = NOW()
		WHERE id = $3 AND state = $4
		RETURNING ` + tradeColumns

	var releaseMode any
	if upd.ReleaseMode != nil {
		releaseMode = string(*upd.ReleaseMode)
	}

	t, err := scanTrade(tx.QueryRowContext(ctx, updateQuery, to, releaseMode, tradeID, from))
	if errors.Is(err, sql.ErrNoRows) {
		// Either the row vanished or it was not in `from` — an illegal transition.
		return entity.EscrowTrade{}, biz.ErrResourceInvalid
	}

	if err != nil {
		return entity.EscrowTrade{}, err
	}

	if err := insertEntries(ctx, tx, entries); err != nil {
		return entity.EscrowTrade{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.EscrowTrade{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.EscrowTrade{}, err
	}

	return t, nil
}

// MarkConsumed implements biz.RepositoryEscrow: insert inboxKey if absent.
func (r *escrowRepo) MarkConsumed(ctx context.Context, inboxKey string) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "escrow.MarkConsumed")
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

// insertEntries appends the append-only ledger rows (never updated/deleted).
func insertEntries(ctx context.Context, tx *sql.Tx, entries []entity.LedgerEntry) error {
	const insert = `
		INSERT INTO escrow_ledger (id, trade_id, participant_account_id, entry_type, amount_cents, ref)
		VALUES ($1,$2,$3,$4,$5,$6)`

	for _, e := range entries {
		if _, err := tx.ExecContext(ctx, insert,
			e.ID, e.TradeID, e.ParticipantAccountID, e.EntryType, e.AmountCents, e.Ref); err != nil {
			return err
		}
	}

	return nil
}

// insertOutbox writes a transactional-outbox row (idempotent on idempotency_key).
func insertOutbox(ctx context.Context, tx *sql.Tx, outbox *entity.OutboxEvent) error {
	if outbox == nil {
		return nil
	}

	const query = `
		INSERT INTO outbox (id, subject, idempotency_key, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING`

	_, err := tx.ExecContext(ctx, query, outbox.ID, outbox.Subject, outbox.IdempotencyKey, outbox.Payload)

	return err
}

// nullableReleaseMode maps an empty release mode to a SQL NULL.
func nullableReleaseMode(m entity.ReleaseMode) any {
	if m == "" {
		return nil
	}

	return string(m)
}
