package biz_test

import (
	"application/internal/biz"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// bid is a tiny helper to build a biz.Bid with a deterministic placed_at offset
// (smaller offset = earlier).
func bid(bidder uuid.UUID, price int64, offsetMs int) biz.Bid {
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	return biz.Bid{
		BidderAccountID: bidder,
		PriceCents:      price,
		PlacedAt:        base.Add(time.Duration(offsetMs) * time.Millisecond),
	}
}

// ---- VICKREY ---------------------------------------------------------------

func TestResolveVickrey(t *testing.T) {
	t.Parallel()

	a := uuid.New()
	b := uuid.New()
	c := uuid.New()

	tests := []struct {
		name        string
		bids        []biz.Bid
		wantWon     bool
		wantWinner  uuid.UUID
		wantCleared int64
	}{
		{
			name:    "empty set -> no winner",
			bids:    nil,
			wantWon: false,
		},
		{
			name:        "single bidder pays own price (no 2nd price)",
			bids:        []biz.Bid{bid(a, 5000, 0)},
			wantWon:     true,
			wantWinner:  a,
			wantCleared: 5000,
		},
		{
			// Spec: winner = bidder of the 2nd-highest DISTINCT price, pays that price.
			name: "winner is the 2nd-highest distinct price's bidder",
			bids: []biz.Bid{
				bid(a, 9000, 0),
				bid(b, 7000, 1), // owns the 2nd-highest distinct price -> winner
				bid(c, 5000, 2),
			},
			wantWon:     true,
			wantWinner:  b,
			wantCleared: 7000,
		},
		{
			name: "all same price -> one distinct price -> earliest wins at that price",
			bids: []biz.Bid{
				bid(c, 6000, 5),
				bid(a, 6000, 1), // earliest owns the only distinct price
				bid(b, 6000, 3),
			},
			wantWon:     true,
			wantWinner:  a,
			wantCleared: 6000,
		},
		{
			// The 2nd-highest distinct price is 6000, owned by c -> c wins.
			name: "tie on highest does not change the 2nd-price winner",
			bids: []biz.Bid{
				bid(b, 9000, 4),
				bid(a, 9000, 1),
				bid(c, 6000, 2), // owns the 2nd-highest distinct price -> winner
			},
			wantWon:     true,
			wantWinner:  c,
			wantCleared: 6000,
		},
		{
			// Two bidders tie on the 2nd-highest distinct price (7000); earliest owns
			// it (b at offset 1) and wins.
			name: "tie on the 2nd price -> earliest owner wins at that price",
			bids: []biz.Bid{
				bid(a, 9000, 0),
				bid(b, 7000, 1), // earliest at 7000 -> owns it -> winner
				bid(c, 7000, 2),
			},
			wantWon:     true,
			wantWinner:  b,
			wantCleared: 7000,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res, err := biz.ResolveVickrey(tt.bids)
			require.NoError(t, err)
			require.Equal(t, tt.wantWon, res.Won)

			if tt.wantWon {
				require.Equal(t, tt.wantWinner, res.WinnerAccountID)
				require.Equal(t, tt.wantCleared, res.ClearedPriceCents)
			}
		})
	}
}

// ---- UNIQBID ---------------------------------------------------------------

func TestResolveUniqBid(t *testing.T) {
	t.Parallel()

	a := uuid.New()
	b := uuid.New()
	c := uuid.New()

	tests := []struct {
		name        string
		bids        []biz.Bid
		wantWon     bool
		wantWinner  uuid.UUID
		wantCleared int64
	}{
		{
			name:    "empty set -> aborted",
			bids:    nil,
			wantWon: false,
		},
		{
			name:        "single bid is trivially unique",
			bids:        []biz.Bid{bid(a, 1000, 0)},
			wantWon:     true,
			wantWinner:  a,
			wantCleared: 1000,
		},
		{
			name: "lowest unique among several",
			bids: []biz.Bid{
				bid(a, 1000, 0),
				bid(b, 1000, 1), // 1000 not unique
				bid(c, 2000, 2), // 2000 unique -> winner
				bid(a, 3000, 3), // 3000 unique but higher
			},
			wantWon:     true,
			wantWinner:  c,
			wantCleared: 2000,
		},
		{
			name: "duplicates eliminate the lowest candidate",
			bids: []biz.Bid{
				bid(a, 500, 0),
				bid(b, 500, 1), // 500 dup
				bid(c, 800, 2), // 800 unique -> winner
			},
			wantWon:     true,
			wantWinner:  c,
			wantCleared: 800,
		},
		{
			name: "no unique price at all -> aborted",
			bids: []biz.Bid{
				bid(a, 1000, 0),
				bid(b, 1000, 1),
				bid(a, 2000, 2),
				bid(b, 2000, 3),
			},
			wantWon: false,
		},
		{
			name: "a bidder's multiple distinct prices: lowest unique still wins",
			bids: []biz.Bid{
				bid(a, 100, 0),
				bid(a, 200, 1),
				bid(b, 100, 2), // 100 dup
				bid(c, 150, 3), // 150 unique and lowest unique
			},
			wantWon:     true,
			wantWinner:  c,
			wantCleared: 150,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res, err := biz.ResolveUniqBid(tt.bids)
			require.NoError(t, err)
			require.Equal(t, tt.wantWon, res.Won)

			if tt.wantWon {
				require.Equal(t, tt.wantWinner, res.WinnerAccountID)
				require.Equal(t, tt.wantCleared, res.ClearedPriceCents)
			}
		})
	}
}

// ---- Property / fuzz over many random bid sets -----------------------------

// TestResolveVickrey_Property asserts, over many seeded-random Vickrey bid sets,
// that the cleared price equals the 2nd-highest DISTINCT price (or the highest if
// only one distinct price exists), and that resolution is deterministic.
func TestResolveVickrey_Property(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(42)) //nolint:gosec

	bidders := make([]uuid.UUID, 6)
	for i := range bidders {
		bidders[i] = uuid.New()
	}

	for iter := 0; iter < 2000; iter++ {
		n := rng.Intn(8) // 0..7 bids
		bids := make([]biz.Bid, 0, n)

		seen := make(map[uuid.UUID]struct{}) // vickrey: one bid per bidder
		for i := 0; i < n; i++ {
			who := bidders[rng.Intn(len(bidders))]
			if _, dup := seen[who]; dup {
				continue
			}

			seen[who] = struct{}{}
			price := int64(rng.Intn(10)+1) * 100

			bids = append(bids, bid(who, price, i))
		}

		res, err := biz.ResolveVickrey(bids)
		require.NoError(t, err)

		// Determinism: same input -> same output.
		res2, _ := biz.ResolveVickrey(bids)
		require.Equal(t, res, res2)

		if len(bids) == 0 {
			require.False(t, res.Won)

			continue
		}

		require.True(t, res.Won)

		// Compute the expected cleared price independently: 2nd-highest distinct,
		// or the single distinct price.
		distinct := map[int64]struct{}{}
		var top, second int64 = -1, -1

		for _, b := range bids {
			distinct[b.PriceCents] = struct{}{}
		}

		for p := range distinct {
			if p > top {
				second = top
				top = p
			} else if p > second {
				second = p
			}
		}

		wantCleared := top
		if len(distinct) >= 2 {
			wantCleared = second
		}

		require.Equal(t, wantCleared, res.ClearedPriceCents, "iter %d bids %v", iter, bids)

		// The winner owns the clearing price: the earliest bidder who placed it.
		var (
			wantWinner   uuid.UUID
			earliestSeen = false
			earliest     biz.Bid
		)

		for _, b := range bids {
			if b.PriceCents != wantCleared {
				continue
			}

			if !earliestSeen || b.PlacedAt.Before(earliest.PlacedAt) {
				earliest = b
				earliestSeen = true
			}
		}

		wantWinner = earliest.BidderAccountID
		require.Equal(t, wantWinner, res.WinnerAccountID, "iter %d", iter)
	}
}

// TestResolveUniqBid_Property asserts, over many seeded-random UniqBid bid sets,
// that when a winner exists the cleared price is chosen by exactly one
// participant and is the minimum such price, and that resolution is deterministic.
func TestResolveUniqBid_Property(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(99)) //nolint:gosec

	bidders := make([]uuid.UUID, 5)
	for i := range bidders {
		bidders[i] = uuid.New()
	}

	for iter := 0; iter < 2000; iter++ {
		n := rng.Intn(10)
		bids := make([]biz.Bid, 0, n)

		// uniqbid: a bidder may submit many DISTINCT prices.
		used := make(map[uuid.UUID]map[int64]struct{})
		for i := 0; i < n; i++ {
			who := bidders[rng.Intn(len(bidders))]
			price := int64(rng.Intn(6) + 1)

			if used[who] == nil {
				used[who] = make(map[int64]struct{})
			}

			if _, dup := used[who][price]; dup {
				continue
			}

			used[who][price] = struct{}{}
			bids = append(bids, bid(who, price, i))
		}

		res, err := biz.ResolveUniqBid(bids)
		require.NoError(t, err)

		res2, _ := biz.ResolveUniqBid(bids)
		require.Equal(t, res, res2)

		// Independently compute distinct-bidder multiplicity per price.
		distinctBidders := map[int64]map[uuid.UUID]struct{}{}
		for _, b := range bids {
			if distinctBidders[b.PriceCents] == nil {
				distinctBidders[b.PriceCents] = map[uuid.UUID]struct{}{}
			}

			distinctBidders[b.PriceCents][b.BidderAccountID] = struct{}{}
		}

		var wantPrice int64 = -1
		for price, set := range distinctBidders {
			if len(set) == 1 && (wantPrice == -1 || price < wantPrice) {
				wantPrice = price
			}
		}

		if wantPrice == -1 {
			require.False(t, res.Won, "iter %d expected abort", iter)

			continue
		}

		require.True(t, res.Won)
		require.Equal(t, wantPrice, res.ClearedPriceCents, "iter %d", iter)
		// The winner is the single bidder who chose the winning price.
		require.Len(t, distinctBidders[res.ClearedPriceCents], 1)
		for who := range distinctBidders[res.ClearedPriceCents] {
			require.Equal(t, who, res.WinnerAccountID)
		}
	}
}
