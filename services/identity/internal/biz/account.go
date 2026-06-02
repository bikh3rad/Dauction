package biz

import (
	"application/internal/entity"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

type account struct {
	logger *slog.Logger
	repo   RepositoryAccount
}

var _ UsecaseAccount = (*account)(nil)

// NewAccount constructs the account use case.
func NewAccount(logger *slog.Logger, repo RepositoryAccount) *account {
	return &account{
		logger: logger.With("layer", "AccountUsecase"),
		repo:   repo,
	}
}

// Get returns the account, creating a GUEST/PENDING record on first sight so the
// gateway always has a tier+KYC read model for an authenticated subject.
func (uc *account) Get(ctx context.Context, id uuid.UUID) (entity.Account, error) {
	return uc.repo.EnsureExists(ctx, id)
}

// GrantVIP elevates an account to VIP (house/admin action). The tier must rise:
// a VIP (or higher) account is an illegal no-op -> ErrResourceInvalid.
func (uc *account) GrantVIP(ctx context.Context, id uuid.UUID) (entity.Account, error) {
	logger := uc.logger.With("method", "GrantVIP", "account", id)

	acc, err := uc.repo.EnsureExists(ctx, id)
	if err != nil {
		return entity.Account{}, err
	}

	if err := assertElevation(acc.Tier, entity.TierVIP); err != nil {
		logger.WarnContext(ctx, "illegal tier transition", "from", acc.Tier, "to", entity.TierVIP)

		return entity.Account{}, err
	}

	// admin grant is a distinct logical write per account+target tier.
	idempotencyKey := fmt.Sprintf("identity:tier:%s:%s", id, entity.TierVIP)

	outbox, err := newTierChangedOutbox(id, acc.Tier, entity.TierVIP, idempotencyKey)
	if err != nil {
		return entity.Account{}, err
	}

	updated, err := uc.repo.SetTierTx(ctx, id, entity.TierVIP, outbox)
	if err != nil {
		return entity.Account{}, err
	}

	logger.InfoContext(ctx, "granted VIP", "from", acc.Tier)

	return updated, nil
}

// ElevateToMember raises a GUEST to MEMBER on invite.redeemed. Idempotent on
// idempotencyKey (inbox); an already MEMBER/VIP account is a no-op success so a
// replayed or out-of-order event never lowers or errors a tier.
func (uc *account) ElevateToMember(ctx context.Context, id uuid.UUID, idempotencyKey string) error {
	logger := uc.logger.With("method", "ElevateToMember", "account", id)

	fresh, err := uc.repo.MarkConsumed(ctx, idempotencyKey)
	if err != nil {
		return err
	}

	if !fresh {
		logger.InfoContext(ctx, "duplicate invite.redeemed ignored", "key", idempotencyKey)

		return nil
	}

	acc, err := uc.repo.EnsureExists(ctx, id)
	if err != nil {
		return err
	}

	// Only GUEST rises to MEMBER; MEMBER/VIP already satisfies the requirement.
	if !acc.Tier.Below(entity.TierMember) {
		logger.InfoContext(ctx, "account already MEMBER+; no elevation", "tier", acc.Tier)

		return nil
	}

	outbox, err := newTierChangedOutbox(id, acc.Tier, entity.TierMember, idempotencyKey)
	if err != nil {
		return err
	}

	if _, err := uc.repo.SetTierTx(ctx, id, entity.TierMember, outbox); err != nil {
		return err
	}

	logger.InfoContext(ctx, "elevated to MEMBER", "from", acc.Tier)

	return nil
}

// ApproveKyc mirrors kyc.approved onto the account, marking it
// participation-eligible. Idempotent on idempotencyKey. Identity emits no event
// for this (kyc owns the kyc.* vocabulary); it only updates its read model.
func (uc *account) ApproveKyc(ctx context.Context, id uuid.UUID, idempotencyKey string) error {
	logger := uc.logger.With("method", "ApproveKyc", "account", id)

	if _, err := uc.repo.EnsureExists(ctx, id); err != nil {
		return err
	}

	if err := uc.repo.SetKycTx(ctx, id, entity.KycApproved, idempotencyKey); err != nil {
		if errors.Is(err, ErrResourceExists) {
			logger.InfoContext(ctx, "duplicate kyc.approved ignored", "key", idempotencyKey)

			return nil
		}

		return err
	}

	logger.InfoContext(ctx, "marked KYC approved")

	return nil
}

// assertElevation enforces the golden rule "tier only rises": the target must be
// strictly above the current tier and both must be valid. Illegal -> ErrResourceInvalid.
func assertElevation(from, to entity.Tier) error {
	if !from.Valid() || !to.Valid() {
		return fmt.Errorf("%w: unknown tier %q->%q", ErrResourceInvalid, from, to)
	}

	if !from.Below(to) {
		return fmt.Errorf("%w: tier cannot drop or stay (%s->%s)", ErrResourceInvalid, from, to)
	}

	return nil
}
