package biz

import (
	"application/internal/entity"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// codeAlphabet excludes ambiguous characters (0/O, 1/I/L) for human-readable codes.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

const codeLength = 10

type invite struct {
	logger     *slog.Logger
	repo       RepositoryInvite
	issueQuota int
}

var _ UsecaseInvite = (*invite)(nil)

// InviteConfig holds tunable invite-service policy.
type InviteConfig struct {
	// IssueQuota is the maximum number of codes a single account may issue.
	// House policy; configurable per deployment. <=0 means unlimited.
	IssueQuota int `koanf:"issueQuota"`
}

// NewInvite constructs the invite use case.
func NewInvite(logger *slog.Logger, repo RepositoryInvite, cfg InviteConfig) *invite {
	return &invite{
		logger:     logger.With("layer", "Invite"),
		repo:       repo,
		issueQuota: cfg.IssueQuota,
	}
}

// generateCode returns a random single-use invite code.
func generateCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	out := make([]byte, codeLength)
	for i, b := range buf {
		out[i] = codeAlphabet[int(b)%len(codeAlphabet)]
	}

	return string(out), nil
}

// Issue implements UsecaseInvite.
func (uc *invite) Issue(ctx context.Context, issuerAccountID string) (entity.Invite, error) {
	logger := uc.logger.With("method", "Issue")

	if issuerAccountID == "" {
		return entity.Invite{}, fmt.Errorf("%w: missing issuer", ErrResourceInvalid)
	}

	if uc.issueQuota > 0 {
		count, err := uc.repo.CountByIssuer(ctx, issuerAccountID)
		if err != nil {
			logger.ErrorContext(ctx, "failed to count issuer invites", "error", err)

			return entity.Invite{}, err
		}

		if count >= uc.issueQuota {
			logger.WarnContext(ctx, "issue quota exceeded", "issuer", issuerAccountID, "quota", uc.issueQuota)

			return entity.Invite{}, fmt.Errorf("%w: issue quota exceeded", ErrResourceInvalid)
		}
	}

	code, err := generateCode()
	if err != nil {
		return entity.Invite{}, err
	}

	inv := entity.Invite{
		ID:              uuid.New(),
		Code:            code,
		IssuerAccountID: issuerAccountID,
		Status:          entity.InviteStatusIssued,
		CreatedAt:       time.Now().UTC(),
	}

	return uc.repo.Create(ctx, inv)
}

// Redeem implements UsecaseInvite. It builds the invite.redeemed outbox payload
// and a producer-stable idempotency key, then delegates the atomic single-use
// redemption (conditional UPDATE + edge + outbox in one tx) to the repo.
func (uc *invite) Redeem(ctx context.Context, code, redeemerAccountID string) (RedeemResult, error) {
	logger := uc.logger.With("method", "Redeem")

	if code == "" || redeemerAccountID == "" {
		return RedeemResult{}, fmt.Errorf("%w: missing code or redeemer", ErrResourceInvalid)
	}

	// Idempotency key is producer-stable for this logical redemption so the event
	// can be deduplicated downstream even if the outbox publisher retries.
	idempotencyKey := fmt.Sprintf("invite.redeemed:%s:%s", code, redeemerAccountID)

	// The payload includes the issuer (chain parent) which the repo fills in from
	// the redeemed row; we marshal here so the repo stays free of event shape.
	payload, err := json.Marshal(map[string]string{
		"code":        code,
		"redeemed_by": redeemerAccountID,
		// issued_by is resolved inside the repo tx from the invite row.
	})
	if err != nil {
		return RedeemResult{}, err
	}

	res, err := uc.repo.Redeem(ctx, code, redeemerAccountID, string(payload), idempotencyKey)
	if err != nil {
		logger.WarnContext(ctx, "redeem failed", "code", code, "error", err)

		return RedeemResult{}, err
	}

	logger.InfoContext(ctx, "invite redeemed", "code", code, "redeemed_by", redeemerAccountID, "issued_by", res.IssuerAccountID)

	return res, nil
}

// List implements UsecaseInvite.
func (uc *invite) List(ctx context.Context, f ListInvitesFilter) ([]entity.Invite, error) {
	if f.Status != "" && !entity.InviteStatus(f.Status).Valid() {
		return nil, fmt.Errorf("%w: unknown status filter", ErrResourceInvalid)
	}

	return uc.repo.List(ctx, f)
}

// Revoke implements UsecaseInvite.
func (uc *invite) Revoke(ctx context.Context, code string) error {
	return uc.repo.SetStatus(ctx, code, entity.InviteStatusRevoked)
}

// Flag implements UsecaseInvite.
func (uc *invite) Flag(ctx context.Context, code string) error {
	return uc.repo.SetStatus(ctx, code, entity.InviteStatusFlagged)
}

// Chain implements UsecaseInvite.
func (uc *invite) Chain(ctx context.Context, accountID string) ([]entity.InviteEdge, error) {
	if accountID == "" {
		return nil, fmt.Errorf("%w: missing account id", ErrResourceInvalid)
	}

	return uc.repo.Chain(ctx, accountID)
}
