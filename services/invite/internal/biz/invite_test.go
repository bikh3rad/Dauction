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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestInvite_Redeem(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name        string
		code        string
		redeemer    string
		setup       func(r *mocks.MockRepositoryInvite)
		wantErr     error
		wantIssuer  string
	}{
		{
			name:     "happy path records chain and returns issuer",
			code:     "ABCDEF1234",
			redeemer: "acct-new",
			setup: func(r *mocks.MockRepositoryInvite) {
				r.EXPECT().
					Redeem(ctx, "ABCDEF1234", "acct-new", mock.Anything, mock.Anything).
					Return(biz.RedeemResult{
						Code:            "ABCDEF1234",
						IssuerAccountID: "acct-issuer",
						RedeemedBy:      "acct-new",
					}, nil)
			},
			wantIssuer: "acct-issuer",
		},
		{
			name:     "double redeem rejected as invalid",
			code:     "USED000000",
			redeemer: "acct-new",
			setup: func(r *mocks.MockRepositoryInvite) {
				r.EXPECT().
					Redeem(ctx, "USED000000", "acct-new", mock.Anything, mock.Anything).
					Return(biz.RedeemResult{}, biz.ErrResourceInvalid)
			},
			wantErr: biz.ErrResourceInvalid,
		},
		{
			name:     "revoked code rejected as invalid",
			code:     "REVOKED000",
			redeemer: "acct-new",
			setup: func(r *mocks.MockRepositoryInvite) {
				r.EXPECT().
					Redeem(ctx, "REVOKED000", "acct-new", mock.Anything, mock.Anything).
					Return(biz.RedeemResult{}, biz.ErrResourceInvalid)
			},
			wantErr: biz.ErrResourceInvalid,
		},
		{
			name:     "missing code rejected without touching repo",
			code:     "",
			redeemer: "acct-new",
			setup:    func(_ *mocks.MockRepositoryInvite) {},
			wantErr:  biz.ErrResourceInvalid,
		},
		{
			name:     "missing redeemer rejected without touching repo",
			code:     "ABCDEF1234",
			redeemer: "",
			setup:    func(_ *mocks.MockRepositoryInvite) {},
			wantErr:  biz.ErrResourceInvalid,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryInvite(t)
			tc.setup(repo)

			uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{IssueQuota: 5})
			res, err := uc.Redeem(ctx, tc.code, tc.redeemer)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantIssuer, res.IssuerAccountID)
			assert.Equal(t, tc.redeemer, res.RedeemedBy)
		})
	}
}

func TestInvite_Issue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("issues a code under quota", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		repo.EXPECT().CountByIssuer(ctx, "acct-1").Return(2, nil)
		repo.EXPECT().
			Create(ctx, mock.MatchedBy(func(inv entity.Invite) bool {
				return inv.IssuerAccountID == "acct-1" &&
					inv.Status == entity.InviteStatusIssued &&
					inv.Code != ""
			})).
			Return(entity.Invite{Code: "NEWCODE123", Status: entity.InviteStatusIssued}, nil)

		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{IssueQuota: 5})
		inv, err := uc.Issue(ctx, "acct-1")
		require.NoError(t, err)
		assert.Equal(t, entity.InviteStatusIssued, inv.Status)
	})

	t.Run("rejects when quota exceeded", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		repo.EXPECT().CountByIssuer(ctx, "acct-1").Return(5, nil)

		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{IssueQuota: 5})
		_, err := uc.Issue(ctx, "acct-1")
		assert.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("unlimited quota skips count", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		repo.EXPECT().
			Create(ctx, mock.Anything).
			Return(entity.Invite{Status: entity.InviteStatusIssued}, nil)

		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{IssueQuota: 0})
		_, err := uc.Issue(ctx, "acct-1")
		require.NoError(t, err)
	})
}

func TestInvite_RevokeAndFlag(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("revoke delegates to SetStatus REVOKED", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		repo.EXPECT().SetStatus(ctx, "CODE", entity.InviteStatusRevoked).Return(nil)

		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{})
		require.NoError(t, uc.Revoke(ctx, "CODE"))
	})

	t.Run("flag delegates to SetStatus FLAGGED", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		repo.EXPECT().SetStatus(ctx, "CODE", entity.InviteStatusFlagged).Return(nil)

		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{})
		require.NoError(t, uc.Flag(ctx, "CODE"))
	})

	t.Run("revoke surfaces invalid-state error", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		repo.EXPECT().
			SetStatus(ctx, "CODE", entity.InviteStatusRevoked).
			Return(biz.ErrResourceInvalid)

		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{})
		assert.ErrorIs(t, uc.Revoke(ctx, "CODE"), biz.ErrResourceInvalid)
	})
}

func TestInvite_List_BadStatusFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := mocks.NewMockRepositoryInvite(t)

	uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{})
	_, err := uc.List(ctx, biz.ListInvitesFilter{Status: "BOGUS"})
	assert.ErrorIs(t, err, biz.ErrResourceInvalid)
}

func TestInvite_Chain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("returns chain edges for an inviter", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		repo.EXPECT().Chain(ctx, "acct-issuer").Return([]entity.InviteEdge{
			{Code: "C1", InviterAccountID: "acct-issuer", InviteeAccountID: "acct-a"},
			{Code: "C2", InviterAccountID: "acct-issuer", InviteeAccountID: "acct-b"},
		}, nil)

		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{})
		edges, err := uc.Chain(ctx, "acct-issuer")
		require.NoError(t, err)
		assert.Len(t, edges, 2)
	})

	t.Run("rejects empty account id", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{})
		_, err := uc.Chain(ctx, "")
		assert.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryInvite(t)
		boom := errors.New("db down")
		repo.EXPECT().Chain(ctx, "acct-issuer").Return(nil, boom)

		uc := biz.NewInvite(testLogger(), repo, biz.InviteConfig{})
		_, err := uc.Chain(ctx, "acct-issuer")
		assert.ErrorIs(t, err, boom)
	})
}
