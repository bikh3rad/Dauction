package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// priceOrigin is the fixed clock origin shared across the price table tests; it
// matches the auction-dutch table so the notifier provably computes identical
// prices to the engine.
var priceOrigin = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func priceParams(ceiling, floor, step, intervalSecs int64, open time.Time) entity.PriceParams {
	return entity.PriceParams{
		CeilingCents:     ceiling,
		FloorCents:       floor,
		DropStepCents:    step,
		DropIntervalSecs: intervalSecs,
		OpenAt:           open,
	}
}

// TestCurrentPrice mirrors auction-dutch's price table (floor clamp, interval
// boundaries, zero-elapsed, zero step/interval) and asserts the notifier's local
// CurrentPrice yields the SAME server-authoritative number the engine validates a
// buy against: current_price(now) = max(floor, ceiling − step·⌊(now−open)/interval⌋).
func TestCurrentPrice(t *testing.T) {
	t.Parallel()

	open := priceOrigin

	tests := []struct {
		name         string
		ceiling      int64
		floor        int64
		step         int64
		intervalSecs int64
		at           time.Time
		want         int64
	}{
		{
			name: "exactly at open -> ceiling (zero elapsed)", ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			at: open, want: 1000,
		},
		{
			name: "before open instant -> ceiling", ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			at: open.Add(-time.Second), want: 1000,
		},
		{
			name: "one second in, before first interval -> ceiling", ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			at: open.Add(time.Second), want: 1000,
		},
		{
			name: "exactly one interval -> one drop", ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			at: open.Add(60 * time.Second), want: 990,
		},
		{
			name: "just before second interval -> still one drop", ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			at: open.Add(119 * time.Second), want: 990,
		},
		{
			name: "two intervals -> two drops", ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			at: open.Add(120 * time.Second), want: 980,
		},
		{
			name: "many intervals clamps at floor", ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			at: open.Add(1000 * time.Minute), want: 100,
		},
		{
			name: "exact floor boundary instant", ceiling: 1000, floor: 100, step: 100, intervalSecs: 60,
			at: open.Add(9 * 60 * time.Second), want: 100,
		},
		{
			name: "one past floor boundary stays at floor", ceiling: 1000, floor: 100, step: 100, intervalSecs: 60,
			at: open.Add(10 * 60 * time.Second), want: 100,
		},
		{
			name: "zero interval treated as no drop -> floor-clamped ceiling", ceiling: 1000, floor: 100, step: 10, intervalSecs: 0,
			at: open.Add(time.Hour), want: 1000,
		},
		{
			name: "zero step -> no drop", ceiling: 1000, floor: 100, step: 0, intervalSecs: 60,
			at: open.Add(time.Hour), want: 1000,
		},
		{
			name: "floor equals ceiling -> constant", ceiling: 500, floor: 500, step: 10, intervalSecs: 60,
			at: open.Add(time.Hour), want: 500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := priceParams(tc.ceiling, tc.floor, tc.step, tc.intervalSecs, open)
			require.Equal(t, tc.want, biz.CurrentPrice(p, tc.at))
		})
	}
}

// TestNextDropAt asserts the next-drop instant the notifier renders countdowns
// from, mirroring the engine's semantics.
func TestNextDropAt(t *testing.T) {
	t.Parallel()

	open := priceOrigin

	t.Run("before open -> first interval after open", func(t *testing.T) {
		t.Parallel()

		p := priceParams(1000, 100, 10, 60, open)
		got := biz.NextDropAt(p, open.Add(-time.Second))
		require.NotNil(t, got)
		require.Equal(t, open.Add(60*time.Second), got.UTC())
	})

	t.Run("mid-run points at the next interval boundary", func(t *testing.T) {
		t.Parallel()

		p := priceParams(1000, 100, 10, 60, open)
		got := biz.NextDropAt(p, open.Add(90*time.Second))
		require.NotNil(t, got)
		require.Equal(t, open.Add(120*time.Second), got.UTC())
	})

	t.Run("at floor -> no further drops", func(t *testing.T) {
		t.Parallel()

		p := priceParams(1000, 100, 100, 60, open)
		require.Nil(t, biz.NextDropAt(p, open.Add(1000*time.Minute)))
	})

	t.Run("zero step -> nil", func(t *testing.T) {
		t.Parallel()

		p := priceParams(1000, 100, 0, 60, open)
		require.Nil(t, biz.NextDropAt(p, open))
	})
}
