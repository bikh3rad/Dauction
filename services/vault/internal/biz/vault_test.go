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

// TestVault_List_DurationMatrix exercises the binding mode/duration matrix
// (CLAUDE.md §6): durationDays is REQUIRED for timed modes and FORBIDDEN for
// DUTCH. Valid combinations transition IN_VAULT -> APPRAISING and emit
// object.listed; invalid combinations are rejected with ErrResourceInvalid
// BEFORE any repo write.
func TestVault_List_DurationMatrix(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	objectID := uuid.New()

	tests := []struct {
		name      string
		mode      entity.AuctionMode
		duration  int
		wantErr   error
		wantWrite bool
	}{
		{name: "dutch no duration ok", mode: entity.AuctionDutch, duration: 0, wantWrite: true},
		{name: "dutch with duration invalid", mode: entity.AuctionDutch, duration: 5, wantErr: biz.ErrResourceInvalid},
		{name: "vickrey 2 days ok", mode: entity.AuctionVickrey, duration: 2, wantWrite: true},
		{name: "vickrey 5 days ok", mode: entity.AuctionVickrey, duration: 5, wantWrite: true},
		{name: "vickrey 7 days ok", mode: entity.AuctionVickrey, duration: 7, wantWrite: true},
		{name: "vickrey no duration invalid", mode: entity.AuctionVickrey, duration: 0, wantErr: biz.ErrResourceInvalid},
		{name: "vickrey bad duration invalid", mode: entity.AuctionVickrey, duration: 3, wantErr: biz.ErrResourceInvalid},
		{name: "uniqbid 7 days ok", mode: entity.AuctionUniqBid, duration: 7, wantWrite: true},
		{name: "uniqbid no duration invalid", mode: entity.AuctionUniqBid, duration: 0, wantErr: biz.ErrResourceInvalid},
		{name: "unknown mode invalid", mode: entity.AuctionMode("ENGLISH"), duration: 2, wantErr: biz.ErrResourceInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryVault(t)

			// Invalid mode/duration is caught before any GetObject lookup.
			if tc.wantErr == nil {
				repo.EXPECT().GetObject(mock.Anything, objectID).
					Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInVault, AppraisedValueCents: 1000}, nil)
				repo.EXPECT().
					TransitionTx(mock.Anything, objectID, entity.ObjectInVault, entity.ObjectAppraising,
						mock.MatchedBy(func(o entity.OutboxEvent) bool {
							return o.Subject == biz.SubjectObjectListed && o.IdempotencyKey != ""
						})).
					Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectAppraising}, nil)
			}

			uc := biz.NewVault(discardLogger(), repo)
			got, err := uc.List(context.Background(), owner, objectID, biz.ListRequest{Mode: tc.mode, DurationDays: tc.duration})

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, entity.ObjectAppraising, got.State)
		})
	}
}

// TestVault_List_OwnerMismatch asserts acting on another member's object is
// rejected with ErrResourceAccessDenied and never writes.
func TestVault_List_OwnerMismatch(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	other := uuid.New()
	objectID := uuid.New()

	repo := mocks.NewMockRepositoryVault(t)
	repo.EXPECT().GetObject(mock.Anything, objectID).
		Return(entity.VaultObject{ID: objectID, OwnerAccountID: other, State: entity.ObjectInVault, AppraisedValueCents: 1000}, nil)

	uc := biz.NewVault(discardLogger(), repo)
	_, err := uc.List(context.Background(), owner, objectID, biz.ListRequest{Mode: entity.AuctionDutch})
	require.ErrorIs(t, err, biz.ErrResourceAccessDenied)
}

// TestVault_List_WrongState rejects listing an object that is not IN_VAULT.
func TestVault_List_WrongState(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	objectID := uuid.New()

	repo := mocks.NewMockRepositoryVault(t)
	repo.EXPECT().GetObject(mock.Anything, objectID).
		Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInAuction, AppraisedValueCents: 1000}, nil)

	uc := biz.NewVault(discardLogger(), repo)
	_, err := uc.List(context.Background(), owner, objectID, biz.ListRequest{Mode: entity.AuctionDutch})
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}

// TestVault_Buyback_Math covers the 50% (CASH) / 85% (CREDIT) payout with
// integer truncation toward zero on odd values (CLAUDE.md §1, money rule).
func TestVault_Buyback_Math(t *testing.T) {
	t.Parallel()

	owner := uuid.New()

	tests := []struct {
		name       string
		appraised  int64
		mode       entity.BuybackMode
		wantPayout int64
		wantLedger bool // CREDIT appends a ledger row + emits credit.changed
	}{
		{name: "cash even", appraised: 1000, mode: entity.BuybackModeCash, wantPayout: 500},
		{name: "cash odd truncates", appraised: 999, mode: entity.BuybackModeCash, wantPayout: 499}, // 999*50/100 = 499
		{name: "credit even", appraised: 1000, mode: entity.BuybackModeCredit, wantPayout: 850, wantLedger: true},
		{name: "credit odd truncates", appraised: 777, mode: entity.BuybackModeCredit, wantPayout: 660, wantLedger: true},       // 777*85/100 = 660.45 -> 660
		{name: "credit small truncates to zero", appraised: 1, mode: entity.BuybackModeCredit, wantPayout: 0, wantLedger: true}, // 1*85/100 = 0
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			objectID := uuid.New()
			repo := mocks.NewMockRepositoryVault(t)
			repo.EXPECT().GetObject(mock.Anything, objectID).
				Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInVault, AppraisedValueCents: tc.appraised}, nil)

			if tc.wantLedger {
				// CREDIT: ledger entry carries the payout; outbox built from balance.
				repo.EXPECT().
					BuybackTx(mock.Anything, objectID,
						mock.MatchedBy(func(e *entity.VaultCreditEntry) bool {
							return e != nil && e.DeltaCents == tc.wantPayout && e.Reason == entity.CreditBuyback && e.AccountID == owner
						}),
						mock.MatchedBy(func(b biz.OutboxBuilder) bool { return b != nil })).
					Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectBoughtBack}, tc.wantPayout, nil)
			} else {
				// CASH: no ledger row, no event.
				repo.EXPECT().
					BuybackTx(mock.Anything, objectID, (*entity.VaultCreditEntry)(nil), mock.MatchedBy(func(b biz.OutboxBuilder) bool { return b == nil })).
					Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectBoughtBack}, int64(0), nil)
			}

			uc := biz.NewVault(discardLogger(), repo)
			res, err := uc.Buyback(context.Background(), owner, objectID, tc.mode)
			require.NoError(t, err)
			require.Equal(t, tc.wantPayout, res.PayoutCents)
			require.Equal(t, entity.ObjectBoughtBack, res.Object.State)
		})
	}
}

// TestVault_Buyback_Errors covers owner mismatch, wrong state, and unknown mode.
func TestVault_Buyback_Errors(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	objectID := uuid.New()

	t.Run("owner mismatch", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().GetObject(mock.Anything, objectID).
			Return(entity.VaultObject{ID: objectID, OwnerAccountID: uuid.New(), State: entity.ObjectInVault, AppraisedValueCents: 1000}, nil)

		uc := biz.NewVault(discardLogger(), repo)
		_, err := uc.Buyback(context.Background(), owner, objectID, entity.BuybackModeCash)
		require.ErrorIs(t, err, biz.ErrResourceAccessDenied)
	})

	t.Run("wrong state", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().GetObject(mock.Anything, objectID).
			Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInAuction, AppraisedValueCents: 1000}, nil)

		uc := biz.NewVault(discardLogger(), repo)
		_, err := uc.Buyback(context.Background(), owner, objectID, entity.BuybackModeCash)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("unknown mode", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryVault(t)

		uc := biz.NewVault(discardLogger(), repo)
		_, err := uc.Buyback(context.Background(), owner, objectID, entity.BuybackMode("WIRE"))
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestVault_AddObject validates the appraised-value guard and the IN_VAULT seed.
func TestVault_AddObject(t *testing.T) {
	t.Parallel()

	owner := uuid.New()

	t.Run("positive value inserts IN_VAULT", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().
			InsertObject(mock.Anything, mock.MatchedBy(func(o entity.VaultObject) bool {
				return o.OwnerAccountID == owner && o.State == entity.ObjectInVault && o.AppraisedValueCents == 5000
			})).
			Return(entity.VaultObject{OwnerAccountID: owner, State: entity.ObjectInVault, AppraisedValueCents: 5000}, nil)

		uc := biz.NewVault(discardLogger(), repo)
		obj, err := uc.AddObject(context.Background(), owner, "Rolex", "desc", 5000)
		require.NoError(t, err)
		require.Equal(t, entity.ObjectInVault, obj.State)
	})

	t.Run("non-positive value rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryVault(t)

		uc := biz.NewVault(discardLogger(), repo)
		_, err := uc.AddObject(context.Background(), owner, "Rolex", "desc", 0)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestVault_View returns the caller's objects and derived credit balance.
func TestVault_View(t *testing.T) {
	t.Parallel()

	owner := uuid.New()

	repo := mocks.NewMockRepositoryVault(t)
	repo.EXPECT().ListObjects(mock.Anything, owner).
		Return([]entity.VaultObject{{OwnerAccountID: owner, State: entity.ObjectInVault}}, nil)
	repo.EXPECT().CreditBalance(mock.Anything, owner).Return(int64(8500), nil)

	uc := biz.NewVault(discardLogger(), repo)
	v, err := uc.View(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, v.Objects, 1)
	require.Equal(t, int64(8500), v.CreditBalanceCents)
}

// TestVault_SettleAuctionCompleted walks the auction.completed consumption:
// SOLD transition, optional Vault-Credit release, idempotent duplicate, unknown
// object, and already-terminal no-ops.
func TestVault_SettleAuctionCompleted(t *testing.T) {
	t.Parallel()

	owner := uuid.New()

	t.Run("cash release marks SOLD without ledger", func(t *testing.T) {
		t.Parallel()

		objectID := uuid.New()
		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().GetObject(mock.Anything, objectID).
			Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInAuction}, nil)
		repo.EXPECT().
			SettleSoldTx(mock.Anything, objectID, "key-1", (*entity.VaultCreditEntry)(nil),
				mock.MatchedBy(func(b biz.OutboxBuilder) bool { return b == nil })).
			Return(int64(0), nil)

		uc := biz.NewVault(discardLogger(), repo)
		require.NoError(t, uc.SettleAuctionCompleted(context.Background(), biz.AuctionCompletedInput{
			ObjectID: objectID, AsVaultCredit: false, IdempotencyKey: "key-1",
		}))
	})

	t.Run("vault-credit release appends ledger + emits credit.changed", func(t *testing.T) {
		t.Parallel()

		objectID := uuid.New()
		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().GetObject(mock.Anything, objectID).
			Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInAuction}, nil)
		repo.EXPECT().
			SettleSoldTx(mock.Anything, objectID, "key-2",
				mock.MatchedBy(func(e *entity.VaultCreditEntry) bool {
					return e != nil && e.DeltaCents == 11000 && e.Reason == entity.CreditAuctionRelease && e.AccountID == owner
				}),
				mock.MatchedBy(func(b biz.OutboxBuilder) bool { return b != nil })).
			Return(int64(11000), nil)

		uc := biz.NewVault(discardLogger(), repo)
		require.NoError(t, uc.SettleAuctionCompleted(context.Background(), biz.AuctionCompletedInput{
			ObjectID: objectID, AsVaultCredit: true, ReleaseCents: 11000, IdempotencyKey: "key-2",
		}))
	})

	t.Run("duplicate event is a no-op success", func(t *testing.T) {
		t.Parallel()

		objectID := uuid.New()
		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().GetObject(mock.Anything, objectID).
			Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInAuction}, nil)
		repo.EXPECT().
			SettleSoldTx(mock.Anything, objectID, "key-dup", (*entity.VaultCreditEntry)(nil),
				mock.MatchedBy(func(b biz.OutboxBuilder) bool { return b == nil })).
			Return(int64(0), biz.ErrResourceExists)

		uc := biz.NewVault(discardLogger(), repo)
		require.NoError(t, uc.SettleAuctionCompleted(context.Background(), biz.AuctionCompletedInput{
			ObjectID: objectID, IdempotencyKey: "key-dup",
		}))
	})

	t.Run("unknown object ignored", func(t *testing.T) {
		t.Parallel()

		objectID := uuid.New()
		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().GetObject(mock.Anything, objectID).
			Return(entity.VaultObject{}, biz.ErrResourceNotFound)

		uc := biz.NewVault(discardLogger(), repo)
		require.NoError(t, uc.SettleAuctionCompleted(context.Background(), biz.AuctionCompletedInput{
			ObjectID: objectID, IdempotencyKey: "key-x",
		}))
	})

	t.Run("already terminal ignored", func(t *testing.T) {
		t.Parallel()

		objectID := uuid.New()
		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().GetObject(mock.Anything, objectID).
			Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectSold}, nil)

		uc := biz.NewVault(discardLogger(), repo)
		require.NoError(t, uc.SettleAuctionCompleted(context.Background(), biz.AuctionCompletedInput{
			ObjectID: objectID, IdempotencyKey: "key-term",
		}))
	})

	t.Run("repo error propagates", func(t *testing.T) {
		t.Parallel()

		boom := errors.New("db down")
		objectID := uuid.New()
		repo := mocks.NewMockRepositoryVault(t)
		repo.EXPECT().GetObject(mock.Anything, objectID).
			Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInAuction}, nil)
		repo.EXPECT().
			SettleSoldTx(mock.Anything, objectID, "key-err", (*entity.VaultCreditEntry)(nil),
				mock.MatchedBy(func(b biz.OutboxBuilder) bool { return b == nil })).
			Return(int64(0), boom)

		uc := biz.NewVault(discardLogger(), repo)
		require.ErrorIs(t, uc.SettleAuctionCompleted(context.Background(), biz.AuctionCompletedInput{
			ObjectID: objectID, IdempotencyKey: "key-err",
		}), boom)
	})
}
