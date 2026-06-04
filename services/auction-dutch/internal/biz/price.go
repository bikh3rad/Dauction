package biz

import (
	"application/internal/entity"
	"time"
)

// Clock is the injected time source. The auction use case depends on this seam
// (rather than calling time.Now directly) so the server-authoritative price and
// hammer decisions are deterministic and testable, mirroring how identity
// injects its uuid/clock seams. The default is wallClock.
type Clock interface {
	Now() time.Time
}

// wallClock is the production Clock backed by time.Now (UTC).
type wallClock struct{}

// NewWallClock returns the real wall-clock time source.
func NewWallClock() Clock { return wallClock{} }

// Now returns the current UTC time.
func (wallClock) Now() time.Time { return time.Now().UTC() }

// CurrentPrice computes the server-authoritative Dutch price at `now` (root
// CLAUDE.md §3):
//
//	current_price(now) = max(floor, ceiling − drop_step·⌊(now − open_at)/interval⌋)
//
// It is a PURE function of the auction's immutable parameters + the clock; the
// client only renders it, the server re-validates it on every buy. Before the
// auction opens (OpenAt nil, or now before OpenAt) the price is the ceiling. A
// non-positive drop interval is treated as "no drop" (price stays at ceiling) to
// avoid a divide-by-zero on malformed params.
func CurrentPrice(a entity.Auction, now time.Time) int64 {
	if a.OpenAt == nil {
		return a.CeilingCents
	}

	openAt := a.OpenAt.UTC()
	now = now.UTC()

	// Before (or exactly at) open: no drops have occurred yet.
	if !now.After(openAt) {
		return a.CeilingCents
	}

	if a.DropIntervalSeconds <= 0 || a.DropStepCents <= 0 {
		return clampFloor(a.CeilingCents, a.FloorCents)
	}

	elapsed := now.Sub(openAt)
	intervals := int64(elapsed / (time.Duration(a.DropIntervalSeconds) * time.Second))

	price := a.CeilingCents - a.DropStepCents*intervals

	return clampFloor(price, a.FloorCents)
}

// NextDropAt returns the instant of the next price drop after `now`, or nil when
// the price has already reached the floor (no further drops) or the auction is
// not open. The notifier renders countdowns from this; it is never authoritative.
func NextDropAt(a entity.Auction, now time.Time) *time.Time {
	if a.OpenAt == nil || a.DropIntervalSeconds <= 0 || a.DropStepCents <= 0 {
		return nil
	}

	// Already at the floor: no more drops.
	if CurrentPrice(a, now) <= a.FloorCents {
		return nil
	}

	openAt := a.OpenAt.UTC()
	now = now.UTC()

	interval := time.Duration(a.DropIntervalSeconds) * time.Second

	var next time.Time
	if !now.After(openAt) {
		next = openAt.Add(interval)
	} else {
		elapsed := now.Sub(openAt)
		intervals := int64(elapsed / interval)
		next = openAt.Add(time.Duration(intervals+1) * interval)
	}

	return &next
}

// clampFloor returns price, floored at floor (price never drops below the
// reserve).
func clampFloor(price, floor int64) int64 {
	if price < floor {
		return floor
	}

	return price
}
