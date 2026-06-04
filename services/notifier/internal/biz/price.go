package biz

import (
	"application/internal/entity"
	"time"
)

// Clock is the injected time source. The notifier depends on this seam (rather
// than calling time.Now directly) so the locally-computed Dutch price and the
// drop ticker are deterministic and testable, mirroring auction-dutch's Clock.
// The default is wallClock.
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
// CLAUDE.md §3) — re-implementing auction-dutch's PURE price function IDENTICALLY
// so the notifier broadcasts the same number the engine validates a buy against:
//
//	current_price(now) = max(floor, ceiling − drop_step·⌊(now − open_at)/interval⌋)
//
// Before open_at the price is the ceiling. A non-positive interval/step is treated
// as "no drop" (price clamped to floor) to avoid a divide-by-zero on malformed
// params. The notifier never decides anything from this — it is a render value.
func CurrentPrice(p entity.PriceParams, now time.Time) int64 {
	openAt := p.OpenAt.UTC()
	now = now.UTC()

	// Before (or exactly at) open: no drops have occurred yet.
	if !now.After(openAt) {
		return p.CeilingCents
	}

	if p.DropIntervalSecs <= 0 || p.DropStepCents <= 0 {
		return clampFloor(p.CeilingCents, p.FloorCents)
	}

	elapsed := now.Sub(openAt)
	intervals := int64(elapsed / (time.Duration(p.DropIntervalSecs) * time.Second))

	price := p.CeilingCents - p.DropStepCents*intervals

	return clampFloor(price, p.FloorCents)
}

// NextDropAt returns the instant of the next price drop after `now`, or nil when
// the price has already reached the floor (no further drops) or params are
// degenerate. The client renders the countdown from this; it is never
// authoritative.
func NextDropAt(p entity.PriceParams, now time.Time) *time.Time {
	if p.DropIntervalSecs <= 0 || p.DropStepCents <= 0 {
		return nil
	}

	// Already at the floor: no more drops.
	if CurrentPrice(p, now) <= p.FloorCents {
		return nil
	}

	openAt := p.OpenAt.UTC()
	now = now.UTC()

	interval := time.Duration(p.DropIntervalSecs) * time.Second

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
