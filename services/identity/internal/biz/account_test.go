package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestAccount_GrantVIP exercises the "tier only rises" rule for the admin grant.
func TestAccount_GrantVIP(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name       string
		current    entity.Tier
		wantErr    error
		wantSetTx  bool // a tier write (+ outbox) must occur
		wantToTier entity.Tier
	}{
		{name: "guest->vip", current: entity.TierGuest, wantSetTx: true, wantToTier: entity.TierVIP},
		{name: "member->vip", current: entity.TierMember, wantSetTx: true, wantToTier: entity.TierVIP},
		{name: "vip->vip is illegal no-op", current: entity.TierVIP, wantErr: biz.ErrResourceInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryAccount(t)
			repo.EXPECT().EnsureExists(mock.Anything, id).
				Return(entity.Account{ID: id, Tier: tc.current, KycStatus: entity.KycPending}, nil)

			if tc.wantSetTx {
				repo.EXPECT().
					SetTierTx(mock.Anything, id, tc.wantToTier, mock.MatchedBy(func(o entity.OutboxEvent) bool {
						return o.Subject == biz.SubjectAccountTierChanged && o.IdempotencyKey != ""
					})).
					Return(entity.Account{ID: id, Tier: tc.wantToTier, KycStatus: entity.KycPending}, nil)
			}

			uc := biz.NewAccount(discardLogger(), repo)
			got, err := uc.GrantVIP(context.Background(), id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantToTier, got.Tier)
		})
	}
}

// TestAccount_ElevateToMember covers invite.redeemed handling: GUEST rises,
// already-elevated tiers are left untouched, and duplicate events are no-ops.
func TestAccount_ElevateToMember(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	const key = "invite.redeemed:abc"

	tests := []struct {
		name        string
		fresh       bool        // MarkConsumed result (false => duplicate)
		current     entity.Tier // only consulted when fresh
		wantSetTier bool
	}{
		{name: "fresh guest elevates", fresh: true, current: entity.TierGuest, wantSetTier: true},
		{name: "fresh member is no-op", fresh: true, current: entity.TierMember, wantSetTier: false},
		{name: "fresh vip is no-op", fresh: true, current: entity.TierVIP, wantSetTier: false},
		{name: "duplicate event ignored", fresh: false, wantSetTier: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryAccount(t)
			repo.EXPECT().MarkConsumed(mock.Anything, key).Return(tc.fresh, nil)

			if tc.fresh {
				repo.EXPECT().EnsureExists(mock.Anything, id).
					Return(entity.Account{ID: id, Tier: tc.current, KycStatus: entity.KycPending}, nil)
			}

			if tc.wantSetTier {
				repo.EXPECT().
					SetTierTx(mock.Anything, id, entity.TierMember, mock.Anything).
					Return(entity.Account{ID: id, Tier: entity.TierMember}, nil)
			}

			uc := biz.NewAccount(discardLogger(), repo)
			require.NoError(t, uc.ElevateToMember(context.Background(), id, key))
		})
	}
}

// TestAccount_ApproveKyc verifies the kyc mirror and idempotent dedup.
func TestAccount_ApproveKyc(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	const key = "kyc.approved:xyz"

	t.Run("first time approves", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAccount(t)
		repo.EXPECT().EnsureExists(mock.Anything, id).
			Return(entity.Account{ID: id, Tier: entity.TierMember, KycStatus: entity.KycPending}, nil)
		repo.EXPECT().SetKycTx(mock.Anything, id, entity.KycApproved, key).Return(nil)

		uc := biz.NewAccount(discardLogger(), repo)
		require.NoError(t, uc.ApproveKyc(context.Background(), id, key))
	})

	t.Run("duplicate is no-op", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAccount(t)
		repo.EXPECT().EnsureExists(mock.Anything, id).
			Return(entity.Account{ID: id, Tier: entity.TierMember, KycStatus: entity.KycApproved}, nil)
		repo.EXPECT().SetKycTx(mock.Anything, id, entity.KycApproved, key).Return(biz.ErrResourceExists)

		uc := biz.NewAccount(discardLogger(), repo)
		require.NoError(t, uc.ApproveKyc(context.Background(), id, key))
	})

	t.Run("repo error propagates", func(t *testing.T) {
		t.Parallel()

		boom := errors.New("db down")
		repo := mocks.NewMockRepositoryAccount(t)
		repo.EXPECT().EnsureExists(mock.Anything, id).
			Return(entity.Account{ID: id, Tier: entity.TierMember}, nil)
		repo.EXPECT().SetKycTx(mock.Anything, id, entity.KycApproved, key).Return(boom)

		uc := biz.NewAccount(discardLogger(), repo)
		require.ErrorIs(t, uc.ApproveKyc(context.Background(), id, key), boom)
	})
}

// TestAccount_Eligibility checks the participation gate derived on the entity.
func TestAccount_Eligibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tier entity.Tier
		kyc  entity.KycState
		want bool
	}{
		{entity.TierGuest, entity.KycApproved, false},
		{entity.TierMember, entity.KycPending, false},
		{entity.TierMember, entity.KycApproved, true},
		{entity.TierVIP, entity.KycApproved, true},
		{entity.TierVIP, entity.KycRejected, false},
	}

	for _, tc := range tests {
		acc := entity.Account{Tier: tc.tier, KycStatus: tc.kyc}
		require.Equalf(t, tc.want, acc.Eligible(), "tier=%s kyc=%s", tc.tier, tc.kyc)
	}
}
