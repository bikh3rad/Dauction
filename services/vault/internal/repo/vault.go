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

type vaultRepo struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryVault = (*vaultRepo)(nil)

// NewVault constructs the vault repository (raw parameterized pgx SQL with an
// OTel tracer, like the template's placeholder repo).
func NewVault(logger *slog.Logger, db *datasource.PostgresDB) *vaultRepo {
	return &vaultRepo{
		logger: logger.With("layer", "VaultRepo"),
		tracer: otel.Tracer("VaultRepo"),
		db:     db,
	}
}

const objectColumns = `id, owner_account_id, title, description, appraised_value_cents, state, created_at, updated_at`

// GetObject implements biz.RepositoryVault.
func (r *vaultRepo) GetObject(ctx context.Context, objectID uuid.UUID) (entity.VaultObject, error) {
	ctx, span := r.tracer.Start(ctx, "vault.GetObject")
	defer span.End()

	const query = `SELECT ` + objectColumns + ` FROM vault_object WHERE id = $1`

	var o entity.VaultObject
	err := r.db.QueryRowContext(ctx, query, objectID).Scan(
		&o.ID, &o.OwnerAccountID, &o.Title, &o.Description,
		&o.AppraisedValueCents, &o.State, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.VaultObject{}, biz.ErrResourceNotFound
	}

	if err != nil {
		r.logger.WarnContext(ctx, "get object failed", "error", err)

		return entity.VaultObject{}, err
	}

	return o, nil
}

// ListObjects implements biz.RepositoryVault.
func (r *vaultRepo) ListObjects(ctx context.Context, owner uuid.UUID) ([]entity.VaultObject, error) {
	ctx, span := r.tracer.Start(ctx, "vault.ListObjects")
	defer span.End()

	const query = `SELECT ` + objectColumns + `
		FROM vault_object WHERE owner_account_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var objects []entity.VaultObject

	for rows.Next() {
		var o entity.VaultObject
		if err := rows.Scan(
			&o.ID, &o.OwnerAccountID, &o.Title, &o.Description,
			&o.AppraisedValueCents, &o.State, &o.CreatedAt, &o.UpdatedAt); err != nil {
			r.logger.WarnContext(ctx, "scan object row failed", "error", err)

			continue
		}

		objects = append(objects, o)
	}

	return objects, rows.Err()
}

// InsertObject implements biz.RepositoryVault.
func (r *vaultRepo) InsertObject(ctx context.Context, obj entity.VaultObject) (entity.VaultObject, error) {
	ctx, span := r.tracer.Start(ctx, "vault.InsertObject")
	defer span.End()

	const query = `
		INSERT INTO vault_object (id, owner_account_id, title, description, appraised_value_cents, state)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + objectColumns

	var o entity.VaultObject
	err := r.db.QueryRowContext(ctx, query,
		obj.ID, obj.OwnerAccountID, obj.Title, obj.Description, obj.AppraisedValueCents, obj.State).
		Scan(&o.ID, &o.OwnerAccountID, &o.Title, &o.Description,
			&o.AppraisedValueCents, &o.State, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		r.logger.WarnContext(ctx, "insert object failed", "error", err)

		return entity.VaultObject{}, err
	}

	return o, nil
}

// CreditBalance implements biz.RepositoryVault: SUM(delta_cents), 0 when empty.
func (r *vaultRepo) CreditBalance(ctx context.Context, account uuid.UUID) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "vault.CreditBalance")
	defer span.End()

	const query = `SELECT COALESCE(SUM(delta_cents), 0) FROM vault_credit_ledger WHERE account_id = $1`

	var balance int64
	if err := r.db.QueryRowContext(ctx, query, account).Scan(&balance); err != nil {
		r.logger.WarnContext(ctx, "credit balance failed", "error", err)

		return 0, err
	}

	return balance, nil
}

// TransitionTx implements biz.RepositoryVault: conditional UPDATE (state = from)
// + outbox row, in one transaction. Not in `from` => ErrResourceInvalid.
func (r *vaultRepo) TransitionTx(
	ctx context.Context,
	objectID uuid.UUID,
	from, to entity.ObjectState,
	outbox entity.OutboxEvent,
) (entity.VaultObject, error) {
	ctx, span := r.tracer.Start(ctx, "vault.TransitionTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.VaultObject{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const updateQuery = `
		UPDATE vault_object SET state = $1, updated_at = NOW()
		WHERE id = $2 AND state = $3
		RETURNING ` + objectColumns

	var o entity.VaultObject
	err = tx.QueryRowContext(ctx, updateQuery, to, objectID, from).Scan(
		&o.ID, &o.OwnerAccountID, &o.Title, &o.Description,
		&o.AppraisedValueCents, &o.State, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// Either the row vanished or it was not in `from` — an illegal transition.
		return entity.VaultObject{}, biz.ErrResourceInvalid
	}

	if err != nil {
		return entity.VaultObject{}, err
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.VaultObject{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.VaultObject{}, err
	}

	return o, nil
}

// ListWithDetailsTx implements biz.RepositoryVault: in one transaction it
// conditionally transitions the object (state = from), writes its category +
// primary language, replaces its translations + media rows, and writes the
// object.listed outbox row. Not in `from` -> ErrResourceInvalid.
func (r *vaultRepo) ListWithDetailsTx(
	ctx context.Context,
	objectID uuid.UUID,
	from, to entity.ObjectState,
	details entity.ListingDetails,
	outbox entity.OutboxEvent,
) (entity.VaultObject, error) {
	ctx, span := r.tracer.Start(ctx, "vault.ListWithDetailsTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.VaultObject{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const updateQuery = `
		UPDATE vault_object
		SET state = $1, category_code = $2, primary_lang = $3, updated_at = NOW()
		WHERE id = $4 AND state = $5
		RETURNING ` + objectColumns

	var o entity.VaultObject
	err = tx.QueryRowContext(ctx, updateQuery,
		to, details.CategoryCode, details.PrimaryLang, objectID, from).Scan(
		&o.ID, &o.OwnerAccountID, &o.Title, &o.Description,
		&o.AppraisedValueCents, &o.State, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.VaultObject{}, biz.ErrResourceInvalid
	}
	if err != nil {
		return entity.VaultObject{}, err
	}

	// Replace translations (idempotent relist): clear then insert the 4 langs.
	if _, err := tx.ExecContext(ctx, `DELETE FROM vault_object_translation WHERE object_id = $1`, objectID); err != nil {
		return entity.VaultObject{}, err
	}
	for _, t := range details.Translations {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO vault_object_translation (object_id, lang, title, description)
			 VALUES ($1, $2, $3, $4)`,
			objectID, t.Lang, t.Title, t.Description); err != nil {
			return entity.VaultObject{}, err
		}
	}

	// Replace media; position is the array index (0 = cover). The DB CHECK
	// (position 0..6) + UNIQUE(object_id, position) enforce the ≤7 invariant.
	if _, err := tx.ExecContext(ctx, `DELETE FROM vault_object_media WHERE object_id = $1`, objectID); err != nil {
		return entity.VaultObject{}, err
	}
	for i, key := range details.ImageRefs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO vault_object_media (object_id, position, storage_key)
			 VALUES ($1, $2, $3)`,
			objectID, i, key); err != nil {
			return entity.VaultObject{}, err
		}
	}

	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return entity.VaultObject{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.VaultObject{}, err
	}

	return o, nil
}

// BuybackTx implements biz.RepositoryVault: IN_VAULT -> BOUGHT_BACK, optional
// ledger row + credit.changed outbox (built from the post-write balance), all in
// one transaction.
func (r *vaultRepo) BuybackTx(
	ctx context.Context,
	objectID uuid.UUID,
	entry *entity.VaultCreditEntry,
	buildOutbox biz.OutboxBuilder,
) (entity.VaultObject, int64, error) {
	ctx, span := r.tracer.Start(ctx, "vault.BuybackTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.VaultObject{}, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	const updateQuery = `
		UPDATE vault_object SET state = $1, updated_at = NOW()
		WHERE id = $2 AND state = $3
		RETURNING ` + objectColumns

	var o entity.VaultObject
	err = tx.QueryRowContext(ctx, updateQuery, entity.ObjectBoughtBack, objectID, entity.ObjectInVault).Scan(
		&o.ID, &o.OwnerAccountID, &o.Title, &o.Description,
		&o.AppraisedValueCents, &o.State, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.VaultObject{}, 0, biz.ErrResourceInvalid
	}

	if err != nil {
		return entity.VaultObject{}, 0, err
	}

	balance, err := applyLedger(ctx, tx, entry, buildOutbox)
	if err != nil {
		return entity.VaultObject{}, 0, err
	}

	if err := tx.Commit(); err != nil {
		return entity.VaultObject{}, 0, err
	}

	return o, balance, nil
}

// SettleSoldTx implements biz.RepositoryVault: record the inbox key, flip
// IN_AUCTION -> SOLD, and optionally append a ledger row + credit.changed
// outbox, in one transaction. Duplicate inboxKey => ErrResourceExists.
func (r *vaultRepo) SettleSoldTx(
	ctx context.Context,
	objectID uuid.UUID,
	inboxKey string,
	entry *entity.VaultCreditEntry,
	buildOutbox biz.OutboxBuilder,
) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "vault.SettleSoldTx")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	const inboxQuery = `
		INSERT INTO consumed_event (idempotency_key) VALUES ($1)
		ON CONFLICT (idempotency_key) DO NOTHING`

	res, err := tx.ExecContext(ctx, inboxQuery, inboxKey)
	if err != nil {
		return 0, err
	}

	if affected, _ := res.RowsAffected(); affected == 0 {
		return 0, biz.ErrResourceExists
	}

	const updateQuery = `
		UPDATE vault_object SET state = $1, updated_at = NOW()
		WHERE id = $2 AND state = $3`

	if _, err := tx.ExecContext(ctx, updateQuery, entity.ObjectSold, objectID, entity.ObjectInAuction); err != nil {
		return 0, err
	}

	balance, err := applyLedger(ctx, tx, entry, buildOutbox)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return balance, nil
}

// MarkConsumed implements biz.RepositoryVault: insert inboxKey if absent.
func (r *vaultRepo) MarkConsumed(ctx context.Context, inboxKey string) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "vault.MarkConsumed")
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

// applyLedger appends the (optional) credit entry, recomputes the account
// balance within the tx, and writes the (optional) credit.changed outbox row.
// Returns the resulting balance (0 when entry is nil).
func applyLedger(
	ctx context.Context,
	tx *sql.Tx,
	entry *entity.VaultCreditEntry,
	buildOutbox biz.OutboxBuilder,
) (int64, error) {
	if entry == nil {
		return 0, nil
	}

	const insertLedger = `
		INSERT INTO vault_credit_ledger (id, account_id, delta_cents, reason, ref_id)
		VALUES ($1, $2, $3, $4, $5)`

	if _, err := tx.ExecContext(ctx, insertLedger,
		entry.ID, entry.AccountID, entry.DeltaCents, entry.Reason, entry.RefID); err != nil {
		return 0, err
	}

	const balanceQuery = `SELECT COALESCE(SUM(delta_cents), 0) FROM vault_credit_ledger WHERE account_id = $1`

	var balance int64
	if err := tx.QueryRowContext(ctx, balanceQuery, entry.AccountID).Scan(&balance); err != nil {
		return 0, err
	}

	if buildOutbox != nil {
		outbox, err := buildOutbox(balance)
		if err != nil {
			return 0, err
		}

		if err := insertOutbox(ctx, tx, outbox); err != nil {
			return 0, err
		}
	}

	return balance, nil
}

// insertOutbox writes a transactional-outbox row (idempotent on idempotency_key).
func insertOutbox(ctx context.Context, tx *sql.Tx, outbox entity.OutboxEvent) error {
	const outboxQuery = `
		INSERT INTO outbox (id, subject, idempotency_key, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING`

	_, err := tx.ExecContext(ctx, outboxQuery,
		outbox.ID, outbox.Subject, outbox.IdempotencyKey, outbox.Payload)

	return err
}
