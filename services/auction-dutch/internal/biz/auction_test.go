package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeClock is a deterministic injected Clock for the use-case tests.
type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

var testNow = time.Date(2026, 6, 1, 12, 5, 0, 0, time.UTC)

// openAuction returns an OPEN auction whose price has descended a known amount by
// testNow (open 5 min earlier; 10/60s drop -> 5 drops -> 1000-50 = 950).
func openAuction(id uuid.UUID) entity.Auction {
	open := testNow.Add(-5 * time.Minute)

	return entity.Auction{
		ID:                  id,
		LotID:               uuid.New(),
		State:               entity.AuctionOpen,
		CeilingCents:        1000,
		FloorCents:          100,
		DropStepCents:       10,
		DropIntervalSeconds: 60,
		OpenAt:              &open,
	}
}

func eligibleParticipant(auctionID, accountID uuid.UUID) entity.Participant {
	return entity.Participant{
		AuctionID:     auctionID,
		AccountID:     accountID,
		KycApproved:   true,
		Tier:          entity.TierMember,
		ReservationSt: entity.ReservationLocked,
		FullLockState: entity.ReservationLocked,
	}
}

// TestBuy_Hammer covers the hammer action: a happy hammer at the server price,
// price re-validation (a stale client price is irrelevant — Buy takes no client
// price), and the first-valid-buy-wins race (second buy after hammer rejected).
func TestBuy_Hammer(t *testing.T) {
	t.Parallel()

	auctionID := uuid.New()
	accountID := uuid.New()

	t.Run("happy hammer at server price", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		a := openAuction(auctionID)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(a, nil)
		repo.EXPECT().GetParticipant(mock.Anything, auctionID, accountID).
			Return(eligibleParticipant(auctionID, accountID), nil)

		hammered := a
		hammered.State = entity.AuctionHammer
		repo.EXPECT().
			HammerTx(mock.Anything, auctionID, accountID, int64(950), mock.Anything,
				mock.MatchedBy(func(o entity.OutboxEvent) bool {
					return o.Subject == biz.SubjectAuctionHammer && o.IdempotencyKey != ""
				})).
			Return(hammered, nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		got, err := uc.Buy(context.Background(), auctionID, accountID)
		require.NoError(t, err)
		require.Equal(t, entity.AuctionHammer, got.State)
	})

	t.Run("second buy after hammer is rejected (race lost)", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		a := openAuction(auctionID)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(a, nil)
		repo.EXPECT().GetParticipant(mock.Anything, auctionID, accountID).
			Return(eligibleParticipant(auctionID, accountID), nil)
		// The conditional UPDATE affected no rows because it is no longer OPEN.
		repo.EXPECT().
			HammerTx(mock.Anything, auctionID, accountID, int64(950), mock.Anything, mock.Anything).
			Return(entity.Auction{}, biz.ErrResourceInvalid)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Buy(context.Background(), auctionID, accountID)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestBuy_NotOpen rejects a buy when the auction is not OPEN.
func TestBuy_NotOpen(t *testing.T) {
	t.Parallel()

	auctionID := uuid.New()
	accountID := uuid.New()

	states := []entity.AuctionState{
		entity.AuctionScheduled, entity.AuctionHammer, entity.AuctionSettling,
		entity.AuctionCompleted, entity.AuctionAborted,
	}

	for _, st := range states {
		t.Run(string(st), func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryAuction(t)
			a := openAuction(auctionID)
			a.State = st
			repo.EXPECT().Get(mock.Anything, auctionID).Return(a, nil)

			uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
			_, err := uc.Buy(context.Background(), auctionID, accountID)
			require.ErrorIs(t, err, biz.ErrResourceInvalid)
		})
	}
}

// TestBuy_Ineligible rejects a buy for each missing prerequisite (KYC, tier,
// deposit lock, full lock) and for a non-participant.
func TestBuy_Ineligible(t *testing.T) {
	t.Parallel()

	auctionID := uuid.New()
	accountID := uuid.New()

	full := eligibleParticipant(auctionID, accountID)

	mutate := map[string]func(p *entity.Participant){
		"kyc not approved":  func(p *entity.Participant) { p.KycApproved = false },
		"tier too low":      func(p *entity.Participant) { p.Tier = entity.TierGuest },
		"deposit unlocked":  func(p *entity.Participant) { p.ReservationSt = entity.ReservationRequested },
		"fulllock unlocked": func(p *entity.Participant) { p.FullLockState = entity.ReservationRequested },
	}

	for name, mut := range mutate {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryAuction(t)
			repo.EXPECT().Get(mock.Anything, auctionID).Return(openAuction(auctionID), nil)
			p := full
			mut(&p)
			repo.EXPECT().GetParticipant(mock.Anything, auctionID, accountID).Return(p, nil)

			uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
			_, err := uc.Buy(context.Background(), auctionID, accountID)
			require.ErrorIs(t, err, biz.ErrResourceInvalid)
		})
	}

	t.Run("not a participant", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(openAuction(auctionID), nil)
		repo.EXPECT().GetParticipant(mock.Anything, auctionID, accountID).
			Return(entity.Participant{}, biz.ErrResourceNotFound)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Buy(context.Background(), auctionID, accountID)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestOpen covers the entry-to-OPEN gate: SCHEDULED + at least one fully-eligible
// participant opens; otherwise rejected.
func TestOpen(t *testing.T) {
	t.Parallel()

	auctionID := uuid.New()

	t.Run("scheduled with an eligible participant opens", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		a := openAuction(auctionID)
		a.State = entity.AuctionScheduled
		a.OpenAt = nil
		repo.EXPECT().Get(mock.Anything, auctionID).Return(a, nil)
		repo.EXPECT().CountEligibleParticipants(mock.Anything, auctionID).Return(1, nil)

		opened := a
		opened.State = entity.AuctionOpen
		opened.OpenAt = &testNow
		repo.EXPECT().
			OpenTx(mock.Anything, auctionID, testNow, mock.MatchedBy(func(o entity.OutboxEvent) bool {
				return o.Subject == biz.SubjectAuctionOpened && o.IdempotencyKey != ""
			})).
			Return(opened, nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		got, err := uc.Open(context.Background(), auctionID)
		require.NoError(t, err)
		require.Equal(t, entity.AuctionOpen, got.State)
	})

	t.Run("no eligible participant rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		a := openAuction(auctionID)
		a.State = entity.AuctionScheduled
		repo.EXPECT().Get(mock.Anything, auctionID).Return(a, nil)
		repo.EXPECT().CountEligibleParticipants(mock.Anything, auctionID).Return(0, nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Open(context.Background(), auctionID)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("not scheduled rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(openAuction(auctionID), nil) // OPEN

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Open(context.Background(), auctionID)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestStateMachine_CompleteAbort covers the admin SETTLING->COMPLETED transition
// and the abort gate (legal from pre-settlement states, illegal from terminal).
func TestStateMachine_CompleteAbort(t *testing.T) {
	t.Parallel()

	auctionID := uuid.New()

	t.Run("settling completes", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		a := openAuction(auctionID)
		a.State = entity.AuctionSettling
		repo.EXPECT().Get(mock.Anything, auctionID).Return(a, nil)

		done := a
		done.State = entity.AuctionCompleted
		repo.EXPECT().
			TransitionTx(mock.Anything, auctionID, entity.AuctionSettling, entity.AuctionCompleted,
				mock.MatchedBy(func(o entity.OutboxEvent) bool { return o.Subject == biz.SubjectAuctionCompleted })).
			Return(done, nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		got, err := uc.Complete(context.Background(), auctionID)
		require.NoError(t, err)
		require.Equal(t, entity.AuctionCompleted, got.State)
	})

	t.Run("complete from non-settling is illegal", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(openAuction(auctionID), nil) // OPEN

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Complete(context.Background(), auctionID)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("abort from OPEN emits ABORTED", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		a := openAuction(auctionID)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(a, nil)

		aborted := a
		aborted.State = entity.AuctionAborted
		repo.EXPECT().
			TransitionTx(mock.Anything, auctionID, entity.AuctionOpen, entity.AuctionAborted, mock.Anything).
			Return(aborted, nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		got, err := uc.Abort(context.Background(), auctionID)
		require.NoError(t, err)
		require.Equal(t, entity.AuctionAborted, got.State)
	})

	t.Run("abort from terminal is illegal", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		a := openAuction(auctionID)
		a.State = entity.AuctionCompleted
		repo.EXPECT().Get(mock.Anything, auctionID).Return(a, nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Abort(context.Background(), auctionID)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestReserve covers the 10% deposit + 100% full-lock requests: eligible request
// creates a REQUESTED reservation + emits escrow.lock_requested; ineligible
// callers and non-SCHEDULED auctions are rejected.
func TestReserve(t *testing.T) {
	t.Parallel()

	auctionID := uuid.New()
	accountID := uuid.New()

	scheduled := func() entity.Auction {
		a := openAuction(auctionID)
		a.State = entity.AuctionScheduled
		a.OpenAt = nil

		return a
	}

	t.Run("eligible deposit reserves 10% and emits lock_requested", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(scheduled(), nil)
		repo.EXPECT().
			ReserveTx(mock.Anything,
				mock.MatchedBy(func(p entity.Participant) bool {
					return p.AccountID == accountID && p.KycApproved && p.Tier == entity.TierMember
				}),
				mock.MatchedBy(func(r entity.Reservation) bool {
					// 10% of ceiling 1000 = 100
					return r.Kind == entity.KindDeposit10 && r.AmountCents == 100 &&
						r.State == entity.ReservationRequested && r.EscrowRef != ""
				}),
				mock.MatchedBy(func(o entity.OutboxEvent) bool {
					return o.Subject == biz.SubjectEscrowLockRequested && o.IdempotencyKey != ""
				})).
			Return(entity.Reservation{ID: uuid.New(), Kind: entity.KindDeposit10, AmountCents: 100, State: entity.ReservationRequested}, nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		res, err := uc.Reserve(context.Background(), auctionID, biz.ReserveInput{
			AccountID: accountID, Tier: entity.TierMember, KycApproved: true,
		})
		require.NoError(t, err)
		require.Equal(t, int64(100), res.AmountCents)
	})

	t.Run("eligible full lock locks 100%", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(scheduled(), nil)
		repo.EXPECT().
			ReserveTx(mock.Anything, mock.Anything,
				mock.MatchedBy(func(r entity.Reservation) bool {
					return r.Kind == entity.KindFullLock && r.AmountCents == 1000
				}),
				mock.Anything).
			Return(entity.Reservation{Kind: entity.KindFullLock, AmountCents: 1000}, nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Lock(context.Background(), auctionID, biz.ReserveInput{
			AccountID: accountID, Tier: entity.TierVIP, KycApproved: true,
		})
		require.NoError(t, err)
	})

	t.Run("ineligible tier rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Reserve(context.Background(), auctionID, biz.ReserveInput{
			AccountID: accountID, Tier: entity.TierGuest, KycApproved: true,
		})
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("kyc not approved rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Reserve(context.Background(), auctionID, biz.ReserveInput{
			AccountID: accountID, Tier: entity.TierMember, KycApproved: false,
		})
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("non-scheduled auction rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().Get(mock.Anything, auctionID).Return(openAuction(auctionID), nil) // OPEN

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		_, err := uc.Reserve(context.Background(), auctionID, biz.ReserveInput{
			AccountID: accountID, Tier: entity.TierMember, KycApproved: true,
		})
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestCreateFromLotScheduled covers lot.scheduled consumption: DUTCH creates a
// SCHEDULED auction, VICKREY/UNIQBID are ignored, a duplicate is an idempotent
// no-op success.
func TestCreateFromLotScheduled(t *testing.T) {
	t.Parallel()

	lotID := uuid.New()
	auctionID := uuid.New()

	t.Run("DUTCH creates a scheduled auction", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().
			CreateAuctionTx(mock.Anything,
				mock.MatchedBy(func(a entity.Auction) bool {
					return a.ID == auctionID && a.LotID == lotID &&
						a.State == entity.AuctionScheduled && a.FloorCents == 50000
				}),
				"sched-key").
			Return(nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		err := uc.CreateFromLotScheduled(context.Background(), biz.LotScheduledInput{
			AuctionID: auctionID, LotID: lotID, Mode: entity.ModeDutch, ReserveCents: 50000,
		}, "sched-key")
		require.NoError(t, err)
	})

	for _, mode := range []entity.AuctionMode{entity.ModeVickrey, entity.ModeUniqBid} {
		t.Run(string(mode)+" is ignored", func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryAuction(t) // no CreateAuctionTx expected
			uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
			err := uc.CreateFromLotScheduled(context.Background(), biz.LotScheduledInput{
				LotID: lotID, Mode: mode, ReserveCents: 50000,
			}, "sched-key")
			require.NoError(t, err)
		})
	}

	t.Run("duplicate is a no-op success", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().CreateAuctionTx(mock.Anything, mock.Anything, "sched-key").
			Return(biz.ErrResourceExists)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		err := uc.CreateFromLotScheduled(context.Background(), biz.LotScheduledInput{
			AuctionID: auctionID, LotID: lotID, Mode: entity.ModeDutch, ReserveCents: 50000,
		}, "sched-key")
		require.NoError(t, err)
	})
}

// TestApplyEscrowLocked covers escrow.locked consumption: flips the reservation,
// an unknown ref is a no-op, and a duplicate is idempotent.
func TestApplyEscrowLocked(t *testing.T) {
	t.Parallel()

	ref := "auction-dutch:lock:a:b:DEPOSIT_10"

	t.Run("flips the matching reservation", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().ApplyEscrowLockedTx(mock.Anything, ref, "inbox-key").Return(nil)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		require.NoError(t, uc.ApplyEscrowLocked(context.Background(),
			biz.EscrowLockedInput{EscrowRef: ref, State: "DEPOSIT_LOCKED"}, "inbox-key"))
	})

	t.Run("unknown ref is a no-op success", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().ApplyEscrowLockedTx(mock.Anything, ref, "inbox-key").
			Return(biz.ErrResourceNotFound)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		require.NoError(t, uc.ApplyEscrowLocked(context.Background(),
			biz.EscrowLockedInput{EscrowRef: ref}, "inbox-key"))
	})

	t.Run("duplicate is idempotent", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t)
		repo.EXPECT().ApplyEscrowLockedTx(mock.Anything, ref, "inbox-key").
			Return(biz.ErrResourceExists)

		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		require.NoError(t, uc.ApplyEscrowLocked(context.Background(),
			biz.EscrowLockedInput{EscrowRef: ref}, "inbox-key"))
	})

	t.Run("empty ref is a no-op success", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryAuction(t) // no call expected
		uc := biz.NewAuction(discardLogger(), repo, fakeClock{testNow})
		require.NoError(t, uc.ApplyEscrowLocked(context.Background(),
			biz.EscrowLockedInput{EscrowRef: ""}, "inbox-key"))
	})
}
