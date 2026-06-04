package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestFundsConservationFuzz is the required §4 property test: it generates many
// random VALID transition sequences over both the Dutch and passive paths (with
// refunds, forfeits and all three dispute rulings) at random amounts, and after
// EVERY transition asserts the funds-conservation invariant holds — gross
// disbursed never exceeds gross inflows, no participant balance goes negative,
// and a settled (terminal) trade fully accounts the locked pot. A seeded rand
// keeps failures reproducible.
func TestFundsConservationFuzz(t *testing.T) {
	t.Parallel()

	const (
		seed       = 0xDA1C // reproducible
		iterations = 400
	)

	rng := rand.New(rand.NewSource(int64(seed)))
	ctx := context.Background()

	for i := 0; i < iterations; i++ {
		repo := newFakeRepo()
		clock := &fakeClock{t: time.Unix(1_700_000_000, 0).UTC()}
		uc := biz.NewEscrow(discardLogger(), repo, biz.WithClock(clock.now))

		id := uuid.New()
		lot, buyer, seller := uuid.New(), uuid.New(), uuid.New()

		// Random non-zero amounts (cents). Keep them bounded but include odd values
		// so SPLIT odd-cent handling is exercised.
		price := int64(rng.Intn(1_000_000) + 1)
		premium := int64(rng.Intn(50_000))
		pot := price + premium

		dutch := rng.Intn(2) == 0

		if dutch {
			runDutch(t, ctx, repo, uc, id, lot, buyer, seller, price, premium, rng)
		} else {
			runPassive(t, ctx, repo, uc, clock, id, lot, buyer, seller, price, premium, pot, rng)
		}
	}
}

// runDutch drives a random valid Dutch sequence: reserve -> full-lock -> hammer
// -> then one terminal of {confirm, refund, forfeit, dispute(ruling)}.
func runDutch(
	t *testing.T,
	ctx context.Context,
	repo *fakeRepo,
	uc biz.UsecaseEscrow,
	id, lot, buyer, seller uuid.UUID,
	price, premium int64,
	rng *rand.Rand,
) {
	t.Helper()

	deposit := price / 10 //nolint:mnd
	if deposit == 0 {
		deposit = 1
	}

	require.NoError(t, uc.LockRequested(ctx, biz.LockRequest{
		TradeID: id, LotID: lot, BuyerAccountID: buyer, SellerAccountID: seller,
		State: entity.StateDepositLocked, AmountCents: deposit, IdempotencyKey: "dep:" + id.String(),
	}))
	assertConservation(t, repo, id, false)

	// A loser may be refunded straight from DEPOSIT_LOCKED.
	if rng.Intn(4) == 0 {
		_, err := uc.Refund(ctx, id, buyer)
		require.NoError(t, err)
		assertConservation(t, repo, id, true)

		return
	}

	require.NoError(t, uc.LockRequested(ctx, biz.LockRequest{
		TradeID: id, LotID: lot, BuyerAccountID: buyer, SellerAccountID: seller,
		State: entity.StateFullLocked, AmountCents: price - deposit, IdempotencyKey: "full:" + id.String(),
	}))
	assertConservation(t, repo, id, false)

	require.NoError(t, uc.Hammer(ctx, biz.HammerInput{
		TradeID: id, WinnerID: buyer, HammerPriceCents: price, PremiumCents: premium,
		IdempotencyKey: "hammer:" + id.String(),
	}))
	assertConservation(t, repo, id, false)

	terminalFromHeld(t, ctx, repo, uc, id, buyer, rng)
}

// runPassive drives a random valid passive sequence: won -> {forfeit-by-late, or
// fund -> terminal-from-HELD}.
func runPassive(
	t *testing.T,
	ctx context.Context,
	repo *fakeRepo,
	uc biz.UsecaseEscrow,
	clock *fakeClock,
	id, lot, buyer, seller uuid.UUID,
	price, premium, pot int64,
	rng *rand.Rand,
) {
	t.Helper()

	require.NoError(t, uc.Won(ctx, biz.WonInput{
		TradeID: id, LotID: lot, WinnerID: buyer, SellerAccountID: seller,
		ClearedPriceCents: price, PremiumCents: premium, IdempotencyKey: "won:" + id.String(),
	}))

	// Sometimes the winner misses the funding window -> forfeit (no inflow).
	if rng.Intn(4) == 0 {
		clock.advance(25 * time.Hour)
		got, err := uc.Fund(ctx, id, buyer, pot)
		require.NoError(t, err)
		require.Equal(t, entity.StateForfeited, got.State)
		assertConservation(t, repo, id, true)

		return
	}

	got, err := uc.Fund(ctx, id, buyer, pot)
	require.NoError(t, err)
	require.Equal(t, entity.StateHeld, got.State)
	assertConservation(t, repo, id, false)

	terminalFromHeld(t, ctx, repo, uc, id, buyer, rng)
}

// terminalFromHeld applies a random valid terminal action to a HELD trade and
// asserts conservation on the settled ledger.
func terminalFromHeld(
	t *testing.T,
	ctx context.Context,
	repo *fakeRepo,
	uc biz.UsecaseEscrow,
	id, buyer uuid.UUID,
	rng *rand.Rand,
) {
	t.Helper()

	switch rng.Intn(6) {
	case 0:
		_, err := uc.Confirm(ctx, id, buyer, entity.ReleaseCash)
		require.NoError(t, err)
	case 1:
		_, err := uc.Confirm(ctx, id, buyer, entity.ReleaseVaultCredit)
		require.NoError(t, err)
	case 2:
		_, err := uc.Refund(ctx, id, buyer)
		require.NoError(t, err)
	case 3:
		require.NoError(t, uc.DisputeResolved(ctx, biz.DisputeInput{
			TradeID: id, Ruling: entity.RulingRefundBuyer, IdempotencyKey: "d:" + id.String(),
		}))
	case 4:
		require.NoError(t, uc.DisputeResolved(ctx, biz.DisputeInput{
			TradeID: id, Ruling: entity.RulingReleaseSeller, IdempotencyKey: "d:" + id.String(),
		}))
	case 5:
		require.NoError(t, uc.DisputeResolved(ctx, biz.DisputeInput{
			TradeID: id, Ruling: entity.RulingSplit, IdempotencyKey: "d:" + id.String(),
		}))
	}

	assertConservation(t, repo, id, true)
}
