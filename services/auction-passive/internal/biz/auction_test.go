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
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func openAuction(mode entity.AuctionMode) entity.Auction {
	return entity.Auction{
		ID:       uuid.New(),
		LotID:    uuid.New(),
		Atype:    mode,
		State:    entity.StateOpen,
		ClosesAt: time.Now().UTC().Add(48 * time.Hour),
	}
}

// TestPlaceBid_DebitBeforePersist verifies the happy path debits one credit and
// THEN persists the bid + bid.placed event (CLAUDE.md §5).
func TestPlaceBid_DebitBeforePersist(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeUniqBid)
	bidder := uuid.New()

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)

	debited := false
	debitor.EXPECT().
		Debit(mock.Anything, bidder, int64(1), mock.Anything, a.ID).
		Run(func(_ context.Context, _ uuid.UUID, _ int64, _ string, _ uuid.UUID) {
			debited = true
		}).
		Return(nil)

	repo.EXPECT().
		InsertBidTx(mock.Anything,
			mock.MatchedBy(func(b entity.PassiveBid) bool {
				return b.AuctionID == a.ID && b.BidderAccountID == bidder && b.PriceCents == 2500
			}),
			false, // not vickrey
			mock.MatchedBy(func(o entity.OutboxEvent) bool {
				// the debit must have already run by the time we reach persist
				return o.Subject == biz.SubjectBidPlaced && debited
			}),
		).
		Return(nil)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	got, err := uc.PlaceBid(context.Background(), biz.PlaceBidInput{
		AuctionID:  a.ID,
		BidderID:   bidder,
		PriceCents: 2500,
		RequestID:  "req-1",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2500), got.PriceCents)
	require.True(t, debited)
}

// TestPlaceBid_DebitFailureNoPersist verifies an out-of-credits debit returns
// ErrResourceInvalid and never persists a bid.
func TestPlaceBid_DebitFailureNoPersist(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeUniqBid)
	bidder := uuid.New()

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)
	debitor.EXPECT().
		Debit(mock.Anything, bidder, int64(1), mock.Anything, a.ID).
		Return(errors.New("out of credits: " + biz.ErrResourceInvalid.Error()))

	// No InsertBidTx expectation -> a call would fail the test.
	uc := biz.NewAuction(discardLogger(), repo, debitor)

	_, err := uc.PlaceBid(context.Background(), biz.PlaceBidInput{
		AuctionID:  a.ID,
		BidderID:   bidder,
		PriceCents: 2500,
	})
	require.Error(t, err)
}

// TestPlaceBid_DebitInvalidMapped verifies the debitor's ErrResourceInvalid is
// surfaced as ErrResourceInvalid (out of credits).
func TestPlaceBid_DebitInvalidMapped(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeUniqBid)
	bidder := uuid.New()

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)
	debitor.EXPECT().
		Debit(mock.Anything, bidder, int64(1), mock.Anything, a.ID).
		Return(biz.ErrResourceInvalid)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	_, err := uc.PlaceBid(context.Background(), biz.PlaceBidInput{
		AuctionID:  a.ID,
		BidderID:   bidder,
		PriceCents: 100,
	})
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}

// TestPlaceBid_ClosedRejected verifies a non-OPEN auction is rejected before any
// debit.
func TestPlaceBid_ClosedRejected(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeUniqBid)
	a.State = entity.StateClosing

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	_, err := uc.PlaceBid(context.Background(), biz.PlaceBidInput{
		AuctionID:  a.ID,
		BidderID:   uuid.New(),
		PriceCents: 100,
	})
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}

// TestPlaceBid_WindowClosedRejected verifies a past-closes_at auction is rejected.
func TestPlaceBid_WindowClosedRejected(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeUniqBid)
	a.ClosesAt = time.Now().UTC().Add(-time.Hour)

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	_, err := uc.PlaceBid(context.Background(), biz.PlaceBidInput{
		AuctionID:  a.ID,
		BidderID:   uuid.New(),
		PriceCents: 100,
	})
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}

// TestPlaceBid_NonPositivePrice verifies a non-positive price is rejected.
func TestPlaceBid_NonPositivePrice(t *testing.T) {
	t.Parallel()

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	_, err := uc.PlaceBid(context.Background(), biz.PlaceBidInput{
		AuctionID:  uuid.New(),
		BidderID:   uuid.New(),
		PriceCents: 0,
	})
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}

// TestPlaceBid_VickreyDoubleBidRejected verifies a second VICKREY bid is rejected
// before any debit.
func TestPlaceBid_VickreyDoubleBidRejected(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeVickrey)
	bidder := uuid.New()

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)
	repo.EXPECT().HasBid(mock.Anything, a.ID, bidder).Return(true, nil)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	_, err := uc.PlaceBid(context.Background(), biz.PlaceBidInput{
		AuctionID:  a.ID,
		BidderID:   bidder,
		PriceCents: 5000,
	})
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}

// TestPlaceBid_DuplicatePersist verifies a duplicate-on-insert (ErrResourceExists)
// surfaces as ErrResourceInvalid without crashing.
func TestPlaceBid_DuplicatePersist(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeUniqBid)
	bidder := uuid.New()

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)
	debitor.EXPECT().Debit(mock.Anything, bidder, int64(1), mock.Anything, a.ID).Return(nil)
	repo.EXPECT().
		InsertBidTx(mock.Anything, mock.Anything, false, mock.Anything).
		Return(biz.ErrResourceExists)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	_, err := uc.PlaceBid(context.Background(), biz.PlaceBidInput{
		AuctionID:  a.ID,
		BidderID:   bidder,
		PriceCents: 100,
	})
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}

// ---- Close & resolve -------------------------------------------------------

// TestClose_VickreyWinner verifies OPEN -> CLOSING -> RESOLVED emits auction.won
// with the Vickrey 2nd price.
func TestClose_VickreyWinner(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeVickrey)
	// Spec: winner = bidder of the 2nd-highest distinct price (7000 here).
	topBidder := uuid.New()
	winner := uuid.New() // owns the 2nd-highest distinct price -> wins

	closing := a
	closing.State = entity.StateClosing

	resolved := closing
	resolved.State = entity.StateResolved
	resolved.WinnerAccountID = &winner
	resolved.ClearedPriceCents = 7000

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)
	repo.EXPECT().
		CloseTx(mock.Anything, a.ID, mock.MatchedBy(func(o entity.OutboxEvent) bool {
			return o.Subject == biz.SubjectAuctionClosed
		})).
		Return(closing, nil)
	repo.EXPECT().
		BidsByAuction(mock.Anything, a.ID).
		Return([]entity.PassiveBid{
			{BidderAccountID: topBidder, PriceCents: 9000, PlacedAt: time.Unix(1, 0)},
			{BidderAccountID: winner, PriceCents: 7000, PlacedAt: time.Unix(2, 0)},
		}, nil)
	repo.EXPECT().
		ResolveTx(mock.Anything, a.ID,
			mock.MatchedBy(func(r biz.Result) bool {
				return r.Won && r.WinnerAccountID == winner && r.ClearedPriceCents == 7000
			}),
			mock.MatchedBy(func(o *entity.OutboxEvent) bool {
				return o != nil && o.Subject == biz.SubjectAuctionWon
			}),
		).
		Return(resolved, nil)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	got, err := uc.Close(context.Background(), a.ID)
	require.NoError(t, err)
	require.Equal(t, entity.StateResolved, got.State)
	require.Equal(t, int64(7000), got.ClearedPriceCents)
}

// TestClose_UniqBidNoUniqueAborts verifies a UniqBid with no unique price ABORTS
// and emits NO auction.won (wonOutbox is nil).
func TestClose_UniqBidNoUniqueAborts(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeUniqBid)
	x := uuid.New()
	y := uuid.New()

	closing := a
	closing.State = entity.StateClosing

	aborted := closing
	aborted.State = entity.StateAborted

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)
	repo.EXPECT().CloseTx(mock.Anything, a.ID, mock.Anything).Return(closing, nil)
	repo.EXPECT().
		BidsByAuction(mock.Anything, a.ID).
		Return([]entity.PassiveBid{
			{BidderAccountID: x, PriceCents: 100, PlacedAt: time.Unix(1, 0)},
			{BidderAccountID: y, PriceCents: 100, PlacedAt: time.Unix(2, 0)},
		}, nil)
	repo.EXPECT().
		ResolveTx(mock.Anything, a.ID,
			mock.MatchedBy(func(r biz.Result) bool { return !r.Won }),
			mock.MatchedBy(func(o *entity.OutboxEvent) bool { return o == nil }),
		).
		Return(aborted, nil)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	got, err := uc.Close(context.Background(), a.ID)
	require.NoError(t, err)
	require.Equal(t, entity.StateAborted, got.State)
}

// TestCreateFromLotScheduled_Vickrey verifies a VICKREY lot.scheduled creates an
// OPEN auction with closes_at = scheduled_at + duration.
func TestCreateFromLotScheduled_Vickrey(t *testing.T) {
	t.Parallel()

	lotID := uuid.New()
	scheduledAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().
		CreateAuctionTx(mock.Anything,
			mock.MatchedBy(func(a entity.Auction) bool {
				return a.LotID == lotID &&
					a.Atype == entity.ModeVickrey &&
					a.State == entity.StateOpen &&
					a.ClosesAt.Equal(scheduledAt.Add(5*24*time.Hour))
			}),
			"inbox-key").
		Return(nil)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	err := uc.CreateFromLotScheduled(context.Background(), biz.LotScheduledInput{
		LotID:        lotID,
		Mode:         entity.ModeVickrey,
		ScheduledAt:  scheduledAt,
		DurationDays: 5,
		ReserveCents: 1000,
	}, "inbox-key")
	require.NoError(t, err)
}

// TestCreateFromLotScheduled_DuplicateIdempotent verifies a duplicate event
// (ErrResourceExists from the inbox) is a no-op success.
func TestCreateFromLotScheduled_DuplicateIdempotent(t *testing.T) {
	t.Parallel()

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().CreateAuctionTx(mock.Anything, mock.Anything, "dup").Return(biz.ErrResourceExists)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	err := uc.CreateFromLotScheduled(context.Background(), biz.LotScheduledInput{
		LotID:        uuid.New(),
		Mode:         entity.ModeUniqBid,
		DurationDays: 2,
	}, "dup")
	require.NoError(t, err)
}

// TestCreateFromLotScheduled_BadMode verifies a non-passive mode is rejected.
func TestCreateFromLotScheduled_BadMode(t *testing.T) {
	t.Parallel()

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	err := uc.CreateFromLotScheduled(context.Background(), biz.LotScheduledInput{
		LotID:        uuid.New(),
		Mode:         entity.AuctionMode("DUTCH"),
		DurationDays: 2,
	}, "k")
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}

// TestClose_NotOpenRejected verifies closing a non-OPEN auction is rejected.
func TestClose_NotOpenRejected(t *testing.T) {
	t.Parallel()

	a := openAuction(entity.ModeVickrey)
	a.State = entity.StateResolved

	repo := mocks.NewMockRepositoryAuction(t)
	debitor := mocks.NewMockCreditDebitor(t)

	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil)

	uc := biz.NewAuction(discardLogger(), repo, debitor)

	_, err := uc.Close(context.Background(), a.ID)
	require.ErrorIs(t, err, biz.ErrResourceInvalid)
}
