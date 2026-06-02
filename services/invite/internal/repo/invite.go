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

const (
	// producerName is stamped on every emitted EventEnvelope.
	producerName = "invite"
	// inviteRedeemedType is the NATS subject for the redemption event.
	inviteRedeemedType = "invite.redeemed"
)

type invite struct {
	logger *slog.Logger
	tracer trace.Tracer
	db     *datasource.PostgresDB
}

var _ biz.RepositoryInvite = (*invite)(nil)

// NewInvite constructs the invite repository.
func NewInvite(logger *slog.Logger, db *datasource.PostgresDB) *invite {
	return &invite{
		logger: logger.With("layer", "InviteRepo"),
		tracer: otel.Tracer("InviteRepo"),
		db:     db,
	}
}

// Create implements biz.RepositoryInvite.
func (r *invite) Create(ctx context.Context, inv entity.Invite) (entity.Invite, error) {
	ctx, span := r.tracer.Start(ctx, "Create")
	defer span.End()

	logger := r.logger.With("method", "Create")
	query := `
		INSERT INTO invite (id, code, issuer_account_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, code, issuer_account_id, status, created_at`

	row := r.db.QueryRowContext(ctx, query,
		inv.ID, inv.Code, inv.IssuerAccountID, string(inv.Status), inv.CreatedAt)

	var out entity.Invite
	var status string
	if err := row.Scan(&out.ID, &out.Code, &out.IssuerAccountID, &status, &out.CreatedAt); err != nil {
		logger.ErrorContext(ctx, "failed to insert invite", "error", err)

		return entity.Invite{}, err
	}
	out.Status = entity.InviteStatus(status)

	return out, nil
}

// CountByIssuer implements biz.RepositoryInvite.
func (r *invite) CountByIssuer(ctx context.Context, issuerAccountID string) (int, error) {
	ctx, span := r.tracer.Start(ctx, "CountByIssuer")
	defer span.End()

	query := `SELECT COUNT(*) FROM invite WHERE issuer_account_id = $1`
	row := r.db.QueryRowContext(ctx, query, issuerAccountID)

	var n int
	if err := row.Scan(&n); err != nil {
		r.logger.ErrorContext(ctx, "failed to count invites", "error", err)

		return 0, err
	}

	return n, nil
}

// Redeem implements biz.RepositoryInvite. Single-use is enforced at the DB level:
// the conditional UPDATE ... WHERE status='ISSUED' affects 0 rows for an already
// consumed/revoked/flagged or missing code, in which case we return
// ErrResourceInvalid. The status flip, chain edge and outbox event commit in one tx.
func (r *invite) Redeem(
	ctx context.Context,
	code, redeemerAccountID, outboxPayload, idempotencyKey string,
) (biz.RedeemResult, error) {
	ctx, span := r.tracer.Start(ctx, "Redeem")
	defer span.End()

	logger := r.logger.With("method", "Redeem")

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return biz.RedeemResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()

	// Conditional, single-use redemption. Returns the issuer only if an ISSUED row
	// was flipped to REDEEMED by this statement.
	var issuer string
	updateQ := `
		UPDATE invite
		SET status = 'REDEEMED', redeemed_at = $2, redeemed_by = $3
		WHERE code = $1 AND status = 'ISSUED'
		RETURNING issuer_account_id`
	err = tx.QueryRowContext(ctx, updateQ, code, now, redeemerAccountID).Scan(&issuer)
	if errors.Is(err, sql.ErrNoRows) {
		// Either the code does not exist or it is not ISSUED (already redeemed /
		// revoked / flagged). Single-use violation -> invalid.
		return biz.RedeemResult{}, fmt.Errorf("%w: code not redeemable", biz.ErrResourceInvalid)
	}
	if err != nil {
		logger.ErrorContext(ctx, "redeem update failed", "error", err)

		return biz.RedeemResult{}, err
	}

	// Record the invite chain edge (inviter -> invitee).
	edgeQ := `
		INSERT INTO invite_edge (id, code, inviter_account_id, invitee_account_id, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err = tx.ExecContext(ctx, edgeQ, uuid.New(), code, issuer, redeemerAccountID, now); err != nil {
		logger.ErrorContext(ctx, "failed to insert invite_edge", "error", err)

		return biz.RedeemResult{}, err
	}

	// Outbox row: written in the same tx as the redemption so the event and the
	// write commit together; the background publisher relays it to NATS.
	// The payload carries issued_by resolved from the redeemed row.
	enrichedPayload := injectIssuer(outboxPayload, issuer)
	outboxQ := `
		INSERT INTO outbox (id, event_id, idempotency_key, producer, type, version, occurred_at, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (idempotency_key) DO NOTHING`
	if _, err = tx.ExecContext(ctx, outboxQ,
		uuid.New(), uuid.NewString(), idempotencyKey, producerName,
		inviteRedeemedType, 1, now, enrichedPayload,
	); err != nil {
		logger.ErrorContext(ctx, "failed to insert outbox row", "error", err)

		return biz.RedeemResult{}, err
	}

	if err = tx.Commit(); err != nil {
		logger.ErrorContext(ctx, "failed to commit redemption", "error", err)

		return biz.RedeemResult{}, err
	}

	return biz.RedeemResult{
		Code:            code,
		IssuerAccountID: issuer,
		RedeemedBy:      redeemerAccountID,
	}, nil
}

// injectIssuer rewrites the JSON payload to include the resolved issued_by field.
// The biz layer marshals code/redeemed_by; the issuer is only known after the tx
// UPDATE, so we splice it in here without re-parsing for the hot path.
func injectIssuer(payload, issuer string) string {
	// payload looks like {"code":"...","redeemed_by":"..."}; insert issued_by.
	const closing = "}"
	if len(payload) == 0 || payload[len(payload)-1:] != closing {
		return payload
	}
	trimmed := payload[:len(payload)-1]

	return fmt.Sprintf(`%s,"issued_by":%q}`, trimmed, issuer)
}

// List implements biz.RepositoryInvite.
func (r *invite) List(ctx context.Context, f biz.ListInvitesFilter) ([]entity.Invite, error) {
	ctx, span := r.tracer.Start(ctx, "List")
	defer span.End()

	logger := r.logger.With("method", "List")

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `
		SELECT id, code, issuer_account_id, status, created_at, redeemed_at, redeemed_by
		FROM invite
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR issuer_account_id = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.QueryContext(ctx, query, f.Status, f.IssuerAccountID, limit, f.Offset)
	if err != nil {
		logger.ErrorContext(ctx, "failed to list invites", "error", err)

		return nil, err
	}
	defer rows.Close()

	var invites []entity.Invite
	for rows.Next() {
		var (
			inv        entity.Invite
			status     string
			redeemedAt sql.NullTime
			redeemedBy sql.NullString
		)
		if err := rows.Scan(&inv.ID, &inv.Code, &inv.IssuerAccountID, &status,
			&inv.CreatedAt, &redeemedAt, &redeemedBy); err != nil {
			logger.WarnContext(ctx, "failed to scan invite row", "error", err)

			continue
		}
		inv.Status = entity.InviteStatus(status)
		if redeemedAt.Valid {
			t := redeemedAt.Time
			inv.RedeemedAt = &t
		}
		if redeemedBy.Valid {
			s := redeemedBy.String
			inv.RedeemedBy = &s
		}
		invites = append(invites, inv)
	}

	if err := rows.Err(); err != nil {
		logger.WarnContext(ctx, "rows iteration error", "error", err)

		return nil, err
	}

	return invites, nil
}

// SetStatus implements biz.RepositoryInvite. Only ISSUED codes may move to
// REVOKED/FLAGGED; a non-ISSUED match -> ErrResourceInvalid, a missing code ->
// ErrResourceNotFound.
func (r *invite) SetStatus(ctx context.Context, code string, target entity.InviteStatus) error {
	ctx, span := r.tracer.Start(ctx, "SetStatus")
	defer span.End()

	logger := r.logger.With("method", "SetStatus")

	query := `UPDATE invite SET status = $2 WHERE code = $1 AND status = 'ISSUED'`
	res, err := r.db.ExecContext(ctx, query, code, string(target))
	if err != nil {
		logger.ErrorContext(ctx, "failed to set status", "error", err)

		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}

	// Distinguish "not found" from "exists but not ISSUED".
	var exists bool
	if err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM invite WHERE code = $1)`, code).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return biz.ErrResourceNotFound
	}

	return fmt.Errorf("%w: invite not in ISSUED state", biz.ErrResourceInvalid)
}

// Chain implements biz.RepositoryInvite.
func (r *invite) Chain(ctx context.Context, accountID string) ([]entity.InviteEdge, error) {
	ctx, span := r.tracer.Start(ctx, "Chain")
	defer span.End()

	logger := r.logger.With("method", "Chain")

	query := `
		SELECT id, code, inviter_account_id, invitee_account_id, created_at
		FROM invite_edge
		WHERE inviter_account_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, accountID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to query invite chain", "error", err)

		return nil, err
	}
	defer rows.Close()

	var edges []entity.InviteEdge
	for rows.Next() {
		var e entity.InviteEdge
		if err := rows.Scan(&e.ID, &e.Code, &e.InviterAccountID, &e.InviteeAccountID, &e.CreatedAt); err != nil {
			logger.WarnContext(ctx, "failed to scan edge row", "error", err)

			continue
		}
		edges = append(edges, e)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return edges, nil
}
