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

// TestWallet_Buy covers the package purchase math (CLAUDE.md §5): each seeded
// package grants the right credits for the right USDC charge, an unknown package is
// a client error, and a successful buy commits a bids.purchased outbox row.
func TestWallet_Buy(t *testing.T) {
	t.Parallel()

	account := uuid.New()

	tests := []struct {
		name        string
		packageID   string
		pkg         entity.BidPackage
		pkgErr      error
		wantGrant   bool
		wantErr     error
		wantCredits int64
		wantCents   int64
	}{
		{
			name:        "PKG_100 -> $80",
			packageID:   "PKG_100",
			pkg:         entity.BidPackage{ID: "PKG_100", Credits: 100, PriceCents: 8000},
			wantGrant:   true,
			wantCredits: 100,
			wantCents:   8000,
		},
		{
			name:        "PKG_50 -> $45",
			packageID:   "PKG_50",
			pkg:         entity.BidPackage{ID: "PKG_50", Credits: 50, PriceCents: 4500},
			wantGrant:   true,
			wantCredits: 50,
			wantCents:   4500,
		},
		{
			name:        "PKG_20 -> $20",
			packageID:   "PKG_20",
			pkg:         entity.BidPackage{ID: "PKG_20", Credits: 20, PriceCents: 2000},
			wantGrant:   true,
			wantCredits: 20,
			wantCents:   2000,
		},
		{
			name:      "unknown package -> invalid",
			packageID: "PKG_NOPE",
			pkgErr:    biz.ErrResourceNotFound,
			wantErr:   biz.ErrResourceInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryWallet(t)
			repo.EXPECT().GetPackage(mock.Anything, tc.packageID).Return(tc.pkg, tc.pkgErr)

			if tc.wantGrant {
				repo.EXPECT().
					GrantTx(mock.Anything,
						mock.MatchedBy(func(p entity.BidPurchase) bool {
							return p.PackageID == tc.pkg.ID &&
								p.CreditsGranted == tc.wantCredits &&
								p.USDCChargedCents == tc.wantCents
						}),
						"idem-1",
						mock.MatchedBy(func(o entity.OutboxEvent) bool {
							return o.Subject == biz.SubjectBidsPurchased && o.IdempotencyKey == "idem-1"
						}),
					).
					Return(biz.PurchaseResult{
						CreditsGranted:   tc.wantCredits,
						USDCChargedCents: tc.wantCents,
						Balance:          tc.wantCredits,
					}, nil)
			}

			uc := biz.NewWallet(discardLogger(), repo)
			res, err := uc.Buy(context.Background(), account, tc.packageID, "idem-1")

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantCredits, res.CreditsGranted)
			require.Equal(t, tc.wantCents, res.USDCChargedCents)
		})
	}
}

// TestWallet_Debit covers the debit-on-bid contract (CLAUDE.md §5): a happy-path
// burn, insufficient balance -> ErrResourceInvalid ("out of credits"), and an
// idempotent replay returning the original debit without a second burn. The biz
// layer delegates the conditional-update + unique-key semantics to DebitTx; these
// tests assert it surfaces each repo outcome correctly.
func TestWallet_Debit(t *testing.T) {
	t.Parallel()

	account := uuid.New()

	t.Run("happy path debits one credit", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryWallet(t)
		repo.EXPECT().
			DebitTx(mock.Anything, mock.MatchedBy(func(d entity.BidDebit) bool {
				return d.AccountID == account && d.AmountCredits == 1 &&
					d.IdempotencyKey == "bid-1" && d.AuctionID == "auction-9"
			})).
			Return(biz.DebitResult{Amount: 1, Balance: 41}, true, nil)

		uc := biz.NewWallet(discardLogger(), repo)
		res, err := uc.Debit(context.Background(), account, 1, "bid-1", "auction-9")

		require.NoError(t, err)
		require.Equal(t, int64(1), res.Amount)
		require.Equal(t, int64(41), res.Balance)
	})

	t.Run("insufficient balance is out of credits", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryWallet(t)
		repo.EXPECT().
			DebitTx(mock.Anything, mock.Anything).
			Return(biz.DebitResult{}, false, biz.ErrResourceInvalid)

		uc := biz.NewWallet(discardLogger(), repo)
		_, err := uc.Debit(context.Background(), account, 1, "bid-2", "auction-9")

		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("idempotent replay returns original, burns nothing", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryWallet(t)
		// fresh=false signals a replay: the original debit + current balance, no burn.
		repo.EXPECT().
			DebitTx(mock.Anything, mock.Anything).
			Return(biz.DebitResult{Amount: 1, Balance: 41}, false, nil)

		uc := biz.NewWallet(discardLogger(), repo)
		res, err := uc.Debit(context.Background(), account, 1, "bid-1", "auction-9")

		require.NoError(t, err)
		require.Equal(t, int64(1), res.Amount)
		require.Equal(t, int64(41), res.Balance)
	})

	t.Run("non-positive amount rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryWallet(t)
		uc := biz.NewWallet(discardLogger(), repo)

		_, err := uc.Debit(context.Background(), account, 0, "bid-z", "auction-9")
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("missing idempotency key rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryWallet(t)
		uc := biz.NewWallet(discardLogger(), repo)

		_, err := uc.Debit(context.Background(), account, 1, "", "auction-9")
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestWallet_Wallet asserts the wallet read is read-through: the stored balance is
// returned verbatim (never recomputed) along with recent activity.
func TestWallet_Wallet(t *testing.T) {
	t.Parallel()

	account := uuid.New()

	repo := mocks.NewMockRepositoryWallet(t)
	repo.EXPECT().GetWallet(mock.Anything, account).
		Return(entity.BidWallet{AccountID: account, BalanceCredits: 42}, nil)
	repo.EXPECT().RecentPurchases(mock.Anything, account, mock.Anything).
		Return([]entity.BidPurchase{{PackageID: "PKG_50", CreditsGranted: 50, USDCChargedCents: 4500}}, nil)
	repo.EXPECT().RecentDebits(mock.Anything, account, mock.Anything).
		Return([]entity.BidDebit{{AmountCredits: 1, IdempotencyKey: "bid-1"}}, nil)

	uc := biz.NewWallet(discardLogger(), repo)
	view, err := uc.Wallet(context.Background(), account, 0)

	require.NoError(t, err)
	require.Equal(t, int64(42), view.Wallet.BalanceCredits)
	require.Len(t, view.Purchases, 1)
	require.Len(t, view.Debits, 1)
}

// TestWallet_Packages asserts the package catalogue is read through from the repo.
func TestWallet_Packages(t *testing.T) {
	t.Parallel()

	seed := []entity.BidPackage{
		{ID: "PKG_100", Credits: 100, PriceCents: 8000, BestValue: true},
		{ID: "PKG_50", Credits: 50, PriceCents: 4500},
		{ID: "PKG_20", Credits: 20, PriceCents: 2000},
	}

	repo := mocks.NewMockRepositoryWallet(t)
	repo.EXPECT().ListPackages(mock.Anything).Return(seed, nil)

	uc := biz.NewWallet(discardLogger(), repo)
	got, err := uc.Packages(context.Background())

	require.NoError(t, err)
	require.Equal(t, seed, got)
}

// TestWallet_Buy_RepoError propagates a tx error from GrantTx unchanged.
func TestWallet_Buy_RepoError(t *testing.T) {
	t.Parallel()

	account := uuid.New()
	boom := errors.New("db down")

	repo := mocks.NewMockRepositoryWallet(t)
	repo.EXPECT().GetPackage(mock.Anything, "PKG_20").
		Return(entity.BidPackage{ID: "PKG_20", Credits: 20, PriceCents: 2000}, nil)
	repo.EXPECT().GrantTx(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(biz.PurchaseResult{}, boom)

	uc := biz.NewWallet(discardLogger(), repo)
	_, err := uc.Buy(context.Background(), account, "PKG_20", "idem-x")

	require.ErrorIs(t, err, boom)
}
