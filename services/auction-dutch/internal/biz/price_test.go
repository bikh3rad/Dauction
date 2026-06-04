package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// open is a fixed clock origin used across the price table tests.
var priceOrigin = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func priceAuction(ceiling, floor, step, intervalSecs int64, open *time.Time) entity.Auction {
	return entity.Auction{
		CeilingCents:        ceiling,
		FloorCents:          floor,
		DropStepCents:       step,
		DropIntervalSeconds: intervalSecs,
		OpenAt:              open,
	}
}

// TestCurrentPrice is the table-driven proof of the server-authoritative price
// function: current_price(now) = max(floor, ceiling − step·⌊(now−open_at)/interval⌋).
func TestCurrentPrice(t *testing.T) {
	t.Parallel()

	open := priceOrigin

	tests := []struct {
		name         string
		ceiling      int64
		floor        int64
		step         int64
		intervalSecs int64
		open         *time.Time
		at           time.Time
		want         int64
	}{
		{
			name:    "not open yet (nil open_at) -> ceiling",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			open: nil,
			at:   open.Add(time.Hour),
			want: 1000,
		},
		{
			name:    "exactly at open -> ceiling (zero elapsed)",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			open: &open,
			at:   open,
			want: 1000,
		},
		{
			name:    "before open instant -> ceiling",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			open: &open,
			at:   open.Add(-time.Second),
			want: 1000,
		},
		{
			name:    "one second in, before first interval -> ceiling (floor of 0)",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			open: &open,
			at:   open.Add(time.Second),
			want: 1000,
		},
		{
			name:    "exactly one interval -> one drop",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			open: &open,
			at:   open.Add(60 * time.Second),
			want: 990,
		},
		{
			name:    "just before second interval -> still one drop",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			open: &open,
			at:   open.Add(119 * time.Second),
			want: 990,
		},
		{
			name:    "two intervals -> two drops",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			open: &open,
			at:   open.Add(120 * time.Second),
			want: 980,
		},
		{
			name:    "many intervals clamps at floor",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 60,
			open: &open,
			at:   open.Add(1000 * time.Minute),
			want: 100,
		},
		{
			name:    "exact floor boundary instant",
			ceiling: 1000, floor: 100, step: 100, intervalSecs: 60,
			open: &open,
			at:   open.Add(9 * 60 * time.Second), // 9 drops of 100 -> 100 == floor
			want: 100,
		},
		{
			name:    "one past floor boundary stays at floor",
			ceiling: 1000, floor: 100, step: 100, intervalSecs: 60,
			open: &open,
			at:   open.Add(10 * 60 * time.Second), // 10 drops -> 0, clamped to 100
			want: 100,
		},
		{
			name:    "zero interval treated as no drop -> floor-clamped ceiling",
			ceiling: 1000, floor: 100, step: 10, intervalSecs: 0,
			open: &open,
			at:   open.Add(time.Hour),
			want: 1000,
		},
		{
			name:    "zero step -> no drop",
			ceiling: 1000, floor: 100, step: 0, intervalSecs: 60,
			open: &open,
			at:   open.Add(time.Hour),
			want: 1000,
		},
		{
			name:    "floor equals ceiling -> constant",
			ceiling: 500, floor: 500, step: 10, intervalSecs: 60,
			open: &open,
			at:   open.Add(time.Hour),
			want: 500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := priceAuction(tc.ceiling, tc.floor, tc.step, tc.intervalSecs, tc.open)
			got := biz.CurrentPrice(a, tc.at)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestNextDropAt asserts the next-drop instant the notifier renders countdowns
// from (never authoritative).
func TestNextDropAt(t *testing.T) {
	t.Parallel()

	open := priceOrigin

	t.Run("nil before open is unset until open arms it", func(t *testing.T) {
		t.Parallel()

		a := priceAuction(1000, 100, 10, 60, &open)
		// before open -> next drop is the first interval after open_at
		got := biz.NextDropAt(a, open.Add(-time.Second))
		require.NotNil(t, got)
		require.Equal(t, open.Add(60*time.Second), got.UTC())
	})

	t.Run("mid-run points at the next interval boundary", func(t *testing.T) {
		t.Parallel()

		a := priceAuction(1000, 100, 10, 60, &open)
		got := biz.NextDropAt(a, open.Add(90*time.Second))
		require.NotNil(t, got)
		require.Equal(t, open.Add(120*time.Second), got.UTC())
	})

	t.Run("at floor -> no further drops", func(t *testing.T) {
		t.Parallel()

		a := priceAuction(1000, 100, 100, 60, &open)
		got := biz.NextDropAt(a, open.Add(1000*time.Minute))
		require.Nil(t, got)
	})

	t.Run("not open (nil) -> nil", func(t *testing.T) {
		t.Parallel()

		a := priceAuction(1000, 100, 10, 60, nil)
		require.Nil(t, biz.NextDropAt(a, open))
	})
}
