package biz

import (
	"application/internal/entity"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Bid is the minimal, immutable projection of a passive_bid that resolution
// needs: who bid, how much, and the server clock tiebreaker. Resolution is a
// PURE function of a slice of these (CLAUDE.md §3) — no I/O, no clock reads — so
// it is fully deterministic and table/fuzz-testable.
type Bid struct {
	BidderAccountID uuid.UUID
	PriceCents      int64
	PlacedAt        time.Time
}

// Result is the outcome of resolving a passive auction. Won is false only for a
// UniqBid with no unique price (the auction ABORTS); in every other case a winner
// and a cleared price are produced.
type Result struct {
	Won               bool
	WinnerAccountID   uuid.UUID
	ClearedPriceCents int64
}

// ResolveVickrey resolves a sealed second-price (Vickrey) auction exactly as the
// Dauction spec states it (root CLAUDE.md §3 / the service brief): order sealed
// bids by price DESC; the **winner is the bidder of the 2nd-highest DISTINCT
// price**, and they pay that 2nd-highest price. Each distinct price is "owned" by
// the earliest bidder who placed it (tie on a price → earliest placed_at).
//
// Edge cases (called out in the spec):
//   - single bidder / one distinct price ⇒ no 2nd price exists, so the owner of
//     that single distinct price wins and pays their own price;
//   - all bids the same price ⇒ one distinct price, so the earliest bidder wins at
//     that price;
//   - empty bid set ⇒ no winner (caller treats as ABORTED).
//
// Determinism: the result is a pure function of the bids' prices + placed_at.
func ResolveVickrey(bids []Bid) (Result, error) {
	if len(bids) == 0 {
		return Result{Won: false}, nil
	}

	// owner[price] = the earliest bidder who placed that price (price ownership is
	// decided by placed_at; ties on a price → earliest owns the price).
	type owner struct {
		bidder   uuid.UUID
		placedAt time.Time
	}

	owners := make(map[int64]owner)
	for _, b := range bids {
		cur, ok := owners[b.PriceCents]
		if !ok || b.PlacedAt.Before(cur.placedAt) {
			owners[b.PriceCents] = owner{bidder: b.BidderAccountID, placedAt: b.PlacedAt}
		}
	}

	// Distinct prices, highest first.
	prices := make([]int64, 0, len(owners))
	for p := range owners {
		prices = append(prices, p)
	}

	sort.Slice(prices, func(i, j int) bool { return prices[i] > prices[j] })

	// The cleared price is the 2nd-highest distinct price; with a single distinct
	// price there is no 2nd price, so the clearing price is that one price.
	clearedIdx := 0
	if len(prices) >= 2 {
		clearedIdx = 1
	}

	clearedPrice := prices[clearedIdx]

	// The winner is the bidder who OWNS the clearing (2nd-highest distinct) price.
	return Result{
		Won:               true,
		WinnerAccountID:   owners[clearedPrice].bidder,
		ClearedPriceCents: clearedPrice,
	}, nil
}

// ResolveUniqBid resolves a lowest-unique-price (UniqBid) auction (CLAUDE.md §3):
// count the multiplicity of each price across ALL bids; among the prices chosen
// by exactly one participant, the MINIMUM wins, and its (single) bidder pays that
// price. A price is "unique" only if exactly one DISTINCT participant chose it —
// the same bidder cannot submit a price twice (the DB enforces distinct prices
// per bidder), but defensively we count distinct bidders per price. With no
// unique price the auction ABORTS (Won=false).
//
// Determinism: the result is a pure function of the bid multiset.
func ResolveUniqBid(bids []Bid) (Result, error) {
	if len(bids) == 0 {
		return Result{Won: false}, nil
	}

	// distinct bidders per price, and the (single) bidder when a price is unique.
	bidders := make(map[int64]map[uuid.UUID]struct{}, len(bids))
	for _, b := range bids {
		set, ok := bidders[b.PriceCents]
		if !ok {
			set = make(map[uuid.UUID]struct{})
			bidders[b.PriceCents] = set
		}

		set[b.BidderAccountID] = struct{}{}
	}

	uniqueWinner := uuid.Nil
	uniquePrice := int64(0)
	found := false

	for price, set := range bidders {
		if len(set) != 1 {
			continue
		}

		if !found || price < uniquePrice {
			found = true
			uniquePrice = price

			for bidder := range set {
				uniqueWinner = bidder
			}
		}
	}

	if !found {
		// No price was chosen by exactly one participant — ABORTED.
		return Result{Won: false}, nil
	}

	return Result{
		Won:               true,
		WinnerAccountID:   uniqueWinner,
		ClearedPriceCents: uniquePrice,
	}, nil
}

// Resolve dispatches to the mode's resolver. It is the single entry point the
// close use case calls; unknown modes are a programming error.
func Resolve(mode entity.AuctionMode, bids []Bid) (Result, error) {
	switch mode {
	case entity.ModeVickrey:
		return ResolveVickrey(bids)
	case entity.ModeUniqBid:
		return ResolveUniqBid(bids)
	default:
		return Result{}, fmt.Errorf("%w: cannot resolve auction mode %q", ErrResourceInvalid, mode)
	}
}
