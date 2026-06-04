package biz_test

import (
	"context"
	"errors"
	"testing"

	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"

	"log/slog"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testAccount = "11111111-1111-1111-1111-111111111111"
)

func memberApproved() entity.Access {
	return entity.Access{ID: testAccount, Tier: entity.TierMember, KycStatus: entity.KycApproved, Eligible: true}
}

func vipApproved() entity.Access {
	return entity.Access{ID: testAccount, Tier: entity.TierVIP, KycStatus: entity.KycApproved, Eligible: true}
}

func guest() entity.Access {
	return entity.Access{ID: testAccount, Tier: entity.TierGuest, KycStatus: entity.KycPending, Eligible: false}
}

func memberPending() entity.Access {
	return entity.Access{ID: testAccount, Tier: entity.TierMember, KycStatus: entity.KycPending, Eligible: false}
}

// TestAuthorize table-drives the gateway guard decision across the requirement
// matrix: public bypass, authed-only, and the participation gate (MEMBER/VIP +
// KYC APPROVED) — including tier-too-low and un-KYC'd rejections.
func TestAuthorize(t *testing.T) {
	t.Parallel()

	participate := biz.RouteRequirement{RequireMember: true, RequireKyc: true}
	authed := biz.RouteRequirement{}
	publicReq := biz.RouteRequirement{Public: true}

	tests := []struct {
		name      string
		accountID string
		req       biz.RouteRequirement
		// access returned by the (mocked) identity read; nil => repo not called.
		access  *entity.Access
		wantErr error
		wantOK  bool
	}{
		{
			name:      "public route bypasses guard (anonymous)",
			accountID: "",
			req:       publicReq,
			access:    nil,
			wantOK:    true,
		},
		{
			name:      "public route bypasses guard even when authed",
			accountID: testAccount,
			req:       publicReq,
			access:    nil,
			wantOK:    true,
		},
		{
			name:      "authed route blocks anonymous",
			accountID: "",
			req:       authed,
			access:    nil,
			wantErr:   biz.ErrResourceAccessDenied,
		},
		{
			name:      "authed route allows any authenticated caller",
			accountID: testAccount,
			req:       authed,
			access:    ptr(guest()),
			wantOK:    true,
		},
		{
			name:      "participation route allows MEMBER+APPROVED",
			accountID: testAccount,
			req:       participate,
			access:    ptr(memberApproved()),
			wantOK:    true,
		},
		{
			name:      "participation route allows VIP+APPROVED",
			accountID: testAccount,
			req:       participate,
			access:    ptr(vipApproved()),
			wantOK:    true,
		},
		{
			name:      "participation route blocks GUEST with TIER_REQUIRED",
			accountID: testAccount,
			req:       participate,
			access:    ptr(guest()),
			wantErr:   biz.ErrTierRequired,
		},
		{
			name:      "participation route blocks un-KYC'd MEMBER with KYC_REQUIRED",
			accountID: testAccount,
			req:       participate,
			access:    ptr(memberPending()),
			wantErr:   biz.ErrKycRequired,
		},
		{
			name:      "participation route blocks anonymous before any lookup",
			accountID: "",
			req:       participate,
			access:    nil,
			wantErr:   biz.ErrResourceAccessDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryAccess(t)
			if tc.access != nil {
				repo.EXPECT().
					FetchAccess(mock.Anything, tc.accountID).
					Return(*tc.access, nil).
					Once()
			}

			uc := biz.NewAccess(slog.Default(), repo)

			res, err := uc.Authorize(context.Background(), tc.accountID, tc.req)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr), "want %v, got %v", tc.wantErr, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantOK, res.Allowed)
		})
	}
}

// TestLookupCaches asserts the access projection is cached: two lookups within
// the TTL hit identity exactly once.
func TestLookupCaches(t *testing.T) {
	t.Parallel()

	repo := mocks.NewMockRepositoryAccess(t)
	repo.EXPECT().
		FetchAccess(mock.Anything, testAccount).
		Return(memberApproved(), nil).
		Once()

	uc := biz.NewAccess(slog.Default(), repo)

	a1, err := uc.Lookup(context.Background(), testAccount)
	require.NoError(t, err)
	require.Equal(t, entity.TierMember, a1.Tier)

	a2, err := uc.Lookup(context.Background(), testAccount)
	require.NoError(t, err)
	require.Equal(t, a1, a2)
}

// TestLookupPropagatesError asserts an upstream failure is surfaced (and not cached).
func TestLookupPropagatesError(t *testing.T) {
	t.Parallel()

	repo := mocks.NewMockRepositoryAccess(t)
	repo.EXPECT().
		FetchAccess(mock.Anything, testAccount).
		Return(entity.Access{}, biz.ErrUpstreamUnavailable).
		Twice()

	uc := biz.NewAccess(slog.Default(), repo)

	_, err := uc.Lookup(context.Background(), testAccount)
	require.ErrorIs(t, err, biz.ErrUpstreamUnavailable)

	// Second call must re-fetch (errors are not cached).
	_, err = uc.Lookup(context.Background(), testAccount)
	require.ErrorIs(t, err, biz.ErrUpstreamUnavailable)
}

func ptr[T any](v T) *T { return &v }
