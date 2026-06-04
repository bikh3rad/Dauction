package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeRepo is an in-memory, behaviourally-faithful RepositoryEscrow used by the
// state-machine and funds-conservation property tests. It actually applies
// conditional transitions and stores the append-only ledger, so the real use
// case drives a real ledger and the invariant can be asserted on it.
type fakeRepo struct {
	mu       sync.Mutex
	trades   map[uuid.UUID]*entity.EscrowTrade
	ledger   map[uuid.UUID][]entity.LedgerEntry
	consumed map[string]bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		trades:   map[uuid.UUID]*entity.EscrowTrade{},
		ledger:   map[uuid.UUID][]entity.LedgerEntry{},
		consumed: map[string]bool{},
	}
}

func (f *fakeRepo) GetTrade(_ context.Context, id uuid.UUID) (entity.EscrowTrade, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, ok := f.trades[id]
	if !ok {
		return entity.EscrowTrade{}, biz.ErrResourceNotFound
	}

	return *t, nil
}

func (f *fakeRepo) ListEntries(_ context.Context, id uuid.UUID) ([]entity.LedgerEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]entity.LedgerEntry(nil), f.ledger[id]...), nil
}

func (f *fakeRepo) Balances(_ context.Context, id uuid.UUID) ([]entity.ParticipantBalance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sums := map[uuid.UUID]int64{}
	for _, e := range f.ledger[id] {
		sums[e.ParticipantAccountID] += e.AmountCents
	}

	out := make([]entity.ParticipantBalance, 0, len(sums))
	for p, c := range sums {
		out = append(out, entity.ParticipantBalance{ParticipantAccountID: p, BalanceCents: c})
	}

	return out, nil
}

func (f *fakeRepo) CreateTradeTx(_ context.Context, trade entity.EscrowTrade, entries []entity.LedgerEntry, _ *entity.OutboxEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.trades[trade.ID]; ok {
		return biz.ErrResourceExists
	}

	cp := trade
	cp.CreatedAt = time.Now()
	cp.UpdatedAt = cp.CreatedAt
	f.trades[trade.ID] = &cp
	f.ledger[trade.ID] = append(f.ledger[trade.ID], entries...)

	return nil
}

func (f *fakeRepo) TransitionTx(
	_ context.Context,
	id uuid.UUID,
	from, to entity.EscrowState,
	upd biz.TradeUpdate,
	entries []entity.LedgerEntry,
	_ *entity.OutboxEvent,
) (entity.EscrowTrade, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, ok := f.trades[id]
	if !ok || t.State != from {
		return entity.EscrowTrade{}, biz.ErrResourceInvalid
	}

	t.State = to
	if upd.ReleaseMode != nil {
		t.ReleaseMode = *upd.ReleaseMode
	}

	t.UpdatedAt = time.Now()
	f.ledger[id] = append(f.ledger[id], entries...)

	return *t, nil
}

func (f *fakeRepo) MarkConsumed(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.consumed[key] {
		return false, nil
	}

	f.consumed[key] = true

	return true, nil
}

// assertConservation asserts the §4 invariant on a trade's real ledger: once
// funds are locked, the gross inflows are >= the gross disbursed, balances are
// never negative, and (when terminal) the pot is fully accounted out.
func assertConservation(t *testing.T, repo *fakeRepo, id uuid.UUID, terminal bool) {
	t.Helper()

	entries, _ := repo.ListEntries(context.Background(), id)
	cons := entity.SummariseConservation(entries)

	require.GreaterOrEqualf(t, cons.Inflows, cons.Disbursed,
		"disbursed %d must never exceed inflows %d", cons.Disbursed, cons.Inflows)

	balances, _ := repo.Balances(context.Background(), id)
	for _, b := range balances {
		require.GreaterOrEqualf(t, b.BalanceCents, int64(0),
			"participant %s balance went negative: %d", b.ParticipantAccountID, b.BalanceCents)
	}

	if terminal {
		require.Truef(t, cons.Balanced(),
			"settled trade must fully account the pot: inflows %d != disbursed %d", cons.Inflows, cons.Disbursed)
	}
}

func newUC(repo biz.RepositoryEscrow, now func() time.Time) biz.UsecaseEscrow {
	if now == nil {
		return biz.NewEscrow(discardLogger(), repo)
	}

	return biz.NewEscrow(discardLogger(), repo, biz.WithClock(now))
}

// seedPassive creates a passive trade UNLOCKED with a future funding deadline.
func seedPassive(t *testing.T, repo *fakeRepo, uc biz.UsecaseEscrow, price, premium int64) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()

	tradeID, lot, winner, seller := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	err := uc.Won(context.Background(), biz.WonInput{
		TradeID:           tradeID,
		LotID:             lot,
		WinnerID:          winner,
		SellerAccountID:   seller,
		ClearedPriceCents: price,
		PremiumCents:      premium,
		IdempotencyKey:    "won:" + tradeID.String(),
	})
	require.NoError(t, err)

	return tradeID, winner, seller
}

// TestFund covers the passive funding path: exact amount required, wrong amount
// rejected, double-fund rejected, late funding -> FORFEITED.
func TestFund(t *testing.T) {
	t.Parallel()

	const price, premium = 100_000, 5_000
	obligation := int64(price + premium)

	t.Run("exact amount funds to HELD", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, price, premium)

		got, err := uc.Fund(context.Background(), id, winner, obligation)
		require.NoError(t, err)
		require.Equal(t, entity.StateHeld, got.State)
		assertConservation(t, repo, id, false)
	})

	t.Run("wrong amount rejected", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, price, premium)

		_, err := uc.Fund(context.Background(), id, winner, obligation-1)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("non-winner rejected", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, _, _ := seedPassive(t, repo, uc, price, premium)

		_, err := uc.Fund(context.Background(), id, uuid.New(), obligation)
		require.ErrorIs(t, err, biz.ErrResourceAccessDenied)
	})

	t.Run("double-fund rejected", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, price, premium)

		_, err := uc.Fund(context.Background(), id, winner, obligation)
		require.NoError(t, err)

		_, err = uc.Fund(context.Background(), id, winner, obligation)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("late funding forfeits", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		// clock starts now; advance past the 24h window before funding.
		base := time.Now().UTC()
		clock := &fakeClock{t: base}
		uc := newUC(repo, clock.now)
		id, winner, _ := seedPassive(t, repo, uc, price, premium)

		clock.advance(25 * time.Hour)

		got, err := uc.Fund(context.Background(), id, winner, obligation)
		require.NoError(t, err)
		require.Equal(t, entity.StateForfeited, got.State)
		assertConservation(t, repo, id, true)
	})
}

// fakeClock is a settable clock for funding-deadline tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t = c.t.Add(d)
}

// TestConfirmRelease covers the cash vs vault-credit release math and the
// dispute-blocks-confirm rule.
func TestConfirmRelease(t *testing.T) {
	t.Parallel()

	t.Run("cash release carves pot, balances to seller", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, 100_000, 5_000)
		_, err := uc.Fund(context.Background(), id, winner, 105_000)
		require.NoError(t, err)

		got, err := uc.Confirm(context.Background(), id, winner, entity.ReleaseCash)
		require.NoError(t, err)
		require.Equal(t, entity.StateReleased, got.State)
		require.Equal(t, entity.ReleaseCash, got.ReleaseMode)

		// Seller received the whole pot (no fees configured): pot - premium carve
		// goes to seller as RELEASE; the PREMIUM carve-out is recorded against the
		// buyer. Conservation: disbursed == inflows.
		assertConservation(t, repo, id, true)
	})

	t.Run("vault credit 110% math", func(t *testing.T) {
		t.Parallel()
		// 110% of 1001 cents truncates toward zero: 1001*110/100 = 1101.
		require.Equal(t, int64(1101), biz.ReleaseCreditCents(1001))
		require.Equal(t, int64(110), biz.ReleaseCreditCents(100))
		require.Equal(t, int64(0), biz.ReleaseCreditCents(0))
	})

	t.Run("non-buyer cannot confirm", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, 100_000, 0)
		_, err := uc.Fund(context.Background(), id, winner, 100_000)
		require.NoError(t, err)

		_, err = uc.Confirm(context.Background(), id, uuid.New(), entity.ReleaseCash)
		require.ErrorIs(t, err, biz.ErrResourceAccessDenied)
	})

	t.Run("confirm requires HELD", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, 100_000, 0) // still UNLOCKED

		_, err := uc.Confirm(context.Background(), id, winner, entity.ReleaseCash)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("dispute blocks confirm", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, 100_000, 0)
		_, err := uc.Fund(context.Background(), id, winner, 100_000)
		require.NoError(t, err)

		// Force DISPUTED directly in the fake.
		repo.trades[id].State = entity.StateDisputed

		_, err = uc.Confirm(context.Background(), id, winner, entity.ReleaseCash)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestDisputeSplit covers SPLIT odd-cent handling: the odd cent goes to the buyer.
func TestDisputeSplit(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := newUC(repo, nil)
	// Odd pot: price 1001, premium 0 -> pot 1001.
	id, winner, _ := seedPassive(t, repo, uc, 1001, 0)
	_, err := uc.Fund(context.Background(), id, winner, 1001)
	require.NoError(t, err)

	err = uc.DisputeResolved(context.Background(), biz.DisputeInput{
		TradeID:        id,
		Ruling:         entity.RulingSplit,
		IdempotencyKey: "dr:" + id.String(),
	})
	require.NoError(t, err)

	trade, _ := repo.GetTrade(context.Background(), id)
	require.Equal(t, entity.StateReleased, trade.State)

	// seller gets floor(1001/2)=500, buyer refund gets the odd cent: 501.
	entries, _ := repo.ListEntries(context.Background(), id)
	var sellerRelease, buyerRefund int64
	for _, e := range entries {
		switch e.EntryType {
		case entity.EntryRelease:
			sellerRelease += e.AmountCents
		case entity.EntryRefund:
			buyerRefund += -e.AmountCents
		}
	}
	require.Equal(t, int64(500), sellerRelease)
	require.Equal(t, int64(501), buyerRefund)
	assertConservation(t, repo, id, true)
}

// TestDisputeRulings covers the three rulings end-to-end.
func TestDisputeRulings(t *testing.T) {
	t.Parallel()

	rulings := []struct {
		name   string
		ruling entity.DisputeRuling
		want   entity.EscrowState
	}{
		{"refund buyer", entity.RulingRefundBuyer, entity.StateRefunded},
		{"release seller", entity.RulingReleaseSeller, entity.StateReleased},
		{"split", entity.RulingSplit, entity.StateReleased},
	}

	for _, tc := range rulings {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := newFakeRepo()
			uc := newUC(repo, nil)
			id, winner, _ := seedPassive(t, repo, uc, 100_000, 0)
			_, err := uc.Fund(context.Background(), id, winner, 100_000)
			require.NoError(t, err)

			err = uc.DisputeResolved(context.Background(), biz.DisputeInput{
				TradeID:        id,
				Ruling:         tc.ruling,
				IdempotencyKey: "dr:" + id.String(),
			})
			require.NoError(t, err)

			trade, _ := repo.GetTrade(context.Background(), id)
			require.Equal(t, tc.want, trade.State)
			assertConservation(t, repo, id, true)
		})
	}
}

// TestDutchPath drives the Dutch reserve -> full-lock -> hammer -> HELD -> release
// flow and asserts conservation throughout.
func TestDutchPath(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := newUC(repo, nil)

	id, lot, buyer, seller := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	// reserve 10% (10_000) -> DEPOSIT_LOCKED
	require.NoError(t, uc.LockRequested(context.Background(), biz.LockRequest{
		TradeID: id, LotID: lot, BuyerAccountID: buyer, SellerAccountID: seller,
		State: entity.StateDepositLocked, AmountCents: 10_000, IdempotencyKey: "lock:dep:" + id.String(),
	}))
	dep, _ := repo.GetTrade(context.Background(), id)
	require.Equal(t, entity.StateDepositLocked, dep.State)

	// full lock remaining 90% (90_000) -> FULL_LOCKED
	require.NoError(t, uc.LockRequested(context.Background(), biz.LockRequest{
		TradeID: id, LotID: lot, BuyerAccountID: buyer, SellerAccountID: seller,
		State: entity.StateFullLocked, AmountCents: 90_000, IdempotencyKey: "lock:full:" + id.String(),
	}))
	full, _ := repo.GetTrade(context.Background(), id)
	require.Equal(t, entity.StateFullLocked, full.State)

	// hammer -> HELD
	require.NoError(t, uc.Hammer(context.Background(), biz.HammerInput{
		TradeID: id, WinnerID: buyer, HammerPriceCents: 100_000, PremiumCents: 0,
		IdempotencyKey: "hammer:" + id.String(),
	}))
	held, _ := repo.GetTrade(context.Background(), id)
	require.Equal(t, entity.StateHeld, held.State)
	assertConservation(t, repo, id, false)

	// buyer confirms -> RELEASED; seller gets the 100_000 pot.
	rel, err := uc.Confirm(context.Background(), id, buyer, entity.ReleaseCash)
	require.NoError(t, err)
	require.Equal(t, entity.StateReleased, rel.State)
	assertConservation(t, repo, id, true)
}

// TestRefundLoser covers the admin/loser refund: locked funds returned, REFUNDED.
func TestRefundLoser(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := newUC(repo, nil)
	id, lot, buyer, seller := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	require.NoError(t, uc.LockRequested(context.Background(), biz.LockRequest{
		TradeID: id, LotID: lot, BuyerAccountID: buyer, SellerAccountID: seller,
		State: entity.StateDepositLocked, AmountCents: 10_000, IdempotencyKey: "lock:dep:" + id.String(),
	}))

	got, err := uc.Refund(context.Background(), id, buyer)
	require.NoError(t, err)
	require.Equal(t, entity.StateRefunded, got.State)
	assertConservation(t, repo, id, true)

	balances, _ := repo.Balances(context.Background(), id)
	for _, b := range balances {
		require.Equal(t, int64(0), b.BalanceCents) // refunded to zero
	}
}

// TestIllegalTransitions asserts illegal jumps are rejected (ErrResourceInvalid)
// or are idempotent no-ops where the consumer dedups.
func TestIllegalTransitions(t *testing.T) {
	t.Parallel()

	t.Run("confirm before fund (UNLOCKED) -> invalid", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, 100, 0)
		_, err := uc.Confirm(context.Background(), id, winner, entity.ReleaseCash)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("refund after release -> invalid", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, 100, 0)
		_, err := uc.Fund(context.Background(), id, winner, 100)
		require.NoError(t, err)
		_, err = uc.Confirm(context.Background(), id, winner, entity.ReleaseCash)
		require.NoError(t, err)

		_, err = uc.Refund(context.Background(), id, winner)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("get on unknown trade -> not found", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		_, err := uc.Get(context.Background(), uuid.New())
		require.ErrorIs(t, err, biz.ErrResourceNotFound)
	})
}

// TestConsumerIdempotency asserts hammer/won/dispute consumption is idempotent on
// replay (no duplicate ledger inflation, terminal state preserved).
func TestConsumerIdempotency(t *testing.T) {
	t.Parallel()

	t.Run("won replay is a no-op", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id := uuid.New()
		in := biz.WonInput{
			TradeID: id, LotID: uuid.New(), WinnerID: uuid.New(), SellerAccountID: uuid.New(),
			ClearedPriceCents: 100, PremiumCents: 0, IdempotencyKey: "won:" + id.String(),
		}
		require.NoError(t, uc.Won(context.Background(), in))
		require.NoError(t, uc.Won(context.Background(), in)) // replay

		entries, _ := repo.ListEntries(context.Background(), id)
		require.Empty(t, entries) // Won creates no ledger rows; replay added none
	})

	t.Run("hammer replay keeps single HELD", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, lot, buyer, seller := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		require.NoError(t, uc.LockRequested(context.Background(), biz.LockRequest{
			TradeID: id, LotID: lot, BuyerAccountID: buyer, SellerAccountID: seller,
			State: entity.StateFullLocked, AmountCents: 100, IdempotencyKey: "lf:" + id.String(),
		}))
		// note: first lock created the trade FULL_LOCKED directly.
		h := biz.HammerInput{TradeID: id, WinnerID: buyer, HammerPriceCents: 100, IdempotencyKey: "h:" + id.String()}
		require.NoError(t, uc.Hammer(context.Background(), h))
		require.NoError(t, uc.Hammer(context.Background(), h)) // replay deduped via inbox

		trade, _ := repo.GetTrade(context.Background(), id)
		require.Equal(t, entity.StateHeld, trade.State)
		assertConservation(t, repo, id, false)
	})

	t.Run("dispute replay is a no-op", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		uc := newUC(repo, nil)
		id, winner, _ := seedPassive(t, repo, uc, 100, 0)
		_, err := uc.Fund(context.Background(), id, winner, 100)
		require.NoError(t, err)

		in := biz.DisputeInput{TradeID: id, Ruling: entity.RulingRefundBuyer, IdempotencyKey: "d:" + id.String()}
		require.NoError(t, uc.DisputeResolved(context.Background(), in))
		require.NoError(t, uc.DisputeResolved(context.Background(), in)) // replay

		trade, _ := repo.GetTrade(context.Background(), id)
		require.Equal(t, entity.StateRefunded, trade.State)
		assertConservation(t, repo, id, true)
	})
}
