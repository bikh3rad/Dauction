package biz

import (
	"application/internal/entity"
	"sync"
)

// Registry is the small in-memory map of currently-open auctions, so a client
// connecting to a room mid-auction gets an accurate snapshot frame immediately.
// It holds NO domain authority — it is a best-effort projection rebuilt from the
// event stream (the durable consumer replays on restart). Concurrency-safe.
type Registry struct {
	mu       sync.RWMutex
	auctions map[string]entity.OpenAuction
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{auctions: make(map[string]entity.OpenAuction)}
}

// Put inserts or replaces an open-auction record.
func (r *Registry) Put(a entity.OpenAuction) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.auctions[a.AuctionID] = a
}

// SetState updates the state of a tracked auction if present.
func (r *Registry) SetState(auctionID, state string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if a, ok := r.auctions[auctionID]; ok {
		a.State = state
		r.auctions[auctionID] = a
	}
}

// Remove drops an auction (it completed/aborted; the room is closing).
func (r *Registry) Remove(auctionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.auctions, auctionID)
}

// Get returns the open-auction record and whether it exists.
func (r *Registry) Get(auctionID string) (entity.OpenAuction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.auctions[auctionID]

	return a, ok
}

// OpenDutch returns the auction ids of every tracked DUTCH auction (the ticker
// loop iterates these to re-broadcast price).
func (r *Registry) OpenDutch() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]string, 0, len(r.auctions))

	for id, a := range r.auctions {
		if a.Mode == entity.ModeDutch && a.State == "OPEN" {
			out = append(out, id)
		}
	}

	return out
}
