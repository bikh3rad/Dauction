package repo

import (
	"application/internal/biz"
	"application/internal/entity"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// GrantRoleTx inserts an account_role row and the account.role_changed outbox
// event in one transaction, then returns the refreshed account. A duplicate grant
// is an idempotent no-op (the event still need not re-fire) -> the outbox row is
// deduped on idempotency_key.
func (r *account) GrantRoleTx(
	ctx context.Context, id uuid.UUID, role entity.Role, grantedBy uuid.UUID, outbox entity.OutboxEvent,
) (entity.Account, error) {
	ctx, span := r.tracer.Start(ctx, "account.GrantRoleTx")
	defer span.End()

	var by any
	if grantedBy != uuid.Nil {
		by = grantedBy
	}

	return r.mutateRoleTx(ctx, id, outbox, `
		INSERT INTO account_role (account_id, role, granted_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id, role) DO NOTHING`,
		id, role, by)
}

// RevokeRoleTx deletes an account_role row + emits account.role_changed.
func (r *account) RevokeRoleTx(
	ctx context.Context, id uuid.UUID, role entity.Role, outbox entity.OutboxEvent,
) (entity.Account, error) {
	ctx, span := r.tracer.Start(ctx, "account.RevokeRoleTx")
	defer span.End()

	return r.mutateRoleTx(ctx, id, outbox,
		`DELETE FROM account_role WHERE account_id = $1 AND role = $2`, id, role)
}

// mutateRoleTx runs a role mutation + outbox emit in one tx and returns the
// refreshed account (with roles loaded).
func (r *account) mutateRoleTx(
	ctx context.Context, id uuid.UUID, outbox entity.OutboxEvent, mutation string, args ...any,
) (entity.Account, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Account{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// The account must exist (FK + a clean 404 rather than an FK violation).
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM account WHERE id = $1`, id).Scan(new(int)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Account{}, biz.ErrResourceNotFound
		}

		return entity.Account{}, err
	}

	if _, err := tx.ExecContext(ctx, mutation, args...); err != nil {
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

	return r.Get(ctx, id)
}

// ListUsers implements admin search/pagination over accounts. Filters are
// optional; an empty filter returns the most recent accounts.
func (r *account) ListUsers(ctx context.Context, f biz.UserFilter) ([]entity.Account, int, error) {
	ctx, span := r.tracer.Start(ctx, "account.ListUsers")
	defer span.End()

	where := []string{"1=1"}
	args := []any{}
	ph := func(val any) string { // append an arg, return its $N placeholder
		args = append(args, val)

		return fmt.Sprintf("$%d", len(args))
	}

	if f.Status != "" {
		where = append(where, "a.status = "+ph(f.Status))
	}
	if f.Role != "" {
		where = append(where,
			"EXISTS (SELECT 1 FROM account_role ar WHERE ar.account_id = a.id AND ar.role = "+ph(f.Role)+")")
	}
	if f.Query != "" {
		where = append(where,
			"(a.handle ILIKE '%' || "+ph(f.Query)+" || '%' OR a.mobile_e164 ILIKE '%' || "+ph(f.Query)+" || '%')")
	}

	clause := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM account a WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT ` + accountColumns + ` FROM account a WHERE ` + clause +
		" ORDER BY a.created_at DESC LIMIT " + ph(limit) + " OFFSET " + ph(f.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var out []entity.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, 0, err
		}
		if a.Roles, err = r.loadRoles(ctx, a.ID); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}

	return out, total, rows.Err()
}

// UpdateUser applies admin profile edits (handle/status). Only non-nil fields are
// written. Tier changes go through the dedicated tier path (events), not here.
func (r *account) UpdateUser(ctx context.Context, id uuid.UUID, p biz.UserPatch) (entity.Account, error) {
	ctx, span := r.tracer.Start(ctx, "account.UpdateUser")
	defer span.End()

	sets := []string{"updated_at = NOW()"}
	args := []any{}
	n := 0
	if p.Handle != nil {
		n++
		sets = append(sets, fmt.Sprintf("handle = $%d", n))
		args = append(args, *p.Handle)
	}
	if p.Status != nil {
		n++
		sets = append(sets, fmt.Sprintf("status = $%d", n))
		args = append(args, string(*p.Status))
	}

	n++
	query := `UPDATE account SET ` + strings.Join(sets, ", ") +
		fmt.Sprintf(" WHERE id = $%d RETURNING ", n) + accountColumns
	args = append(args, id)

	a, err := scanAccount(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Account{}, biz.ErrResourceNotFound
	}
	if err != nil {
		return entity.Account{}, err
	}
	if a.Roles, err = r.loadRoles(ctx, id); err != nil {
		return entity.Account{}, err
	}

	return a, nil
}
