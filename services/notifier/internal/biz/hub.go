package biz

import (
	"application/internal/entity"
	"log/slog"
	"sync"
)

// clientBuffer is the bounded per-client send queue depth. Backpressure policy:
// each client has its own buffered channel; when it is full (a slow/stalled SSE
// reader) the hub DROPS the OLDEST queued frame and enqueues the newest, so one
// slow client can never block the hub or other clients. For a live price feed the
// newest tick is the only one that matters, making drop-oldest the correct choice.
const clientBuffer = 16

// Client is a single connected subscriber (one SSE request). The transport
// (handler) drains C and writes each Message to the wire; on disconnect it calls
// the unregister func the hub returned.
type Client struct {
	C    chan entity.Message
	hub  *Hub
	room string
}

// Hub is the in-memory subscription fan-out. Rooms are keyed by string:
// "auctions/{id}" for an auction feed and "me/{accountId}" for a per-account feed.
// It holds NO domain state — only ephemeral connections — and is concurrency-safe
// via a single mutex guarding the room map. Fan-out itself is non-blocking
// (drop-oldest per client), so holding the lock during Broadcast is cheap.
type Hub struct {
	logger *slog.Logger
	mu     sync.RWMutex
	rooms  map[string]map[*Client]struct{}
}

// NewHub constructs an empty hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		logger: logger.With("layer", "Hub"),
		rooms:  make(map[string]map[*Client]struct{}),
	}
}

// AuctionRoom is the room key for an auction's live feed.
func AuctionRoom(auctionID string) string { return "auctions/" + auctionID }

// MeRoom is the room key for a per-account feed (escrow state, personal toasts).
func MeRoom(accountID string) string { return "me/" + accountID }

// Register adds a client to `room` and returns it. The caller drains Client.C and
// MUST call Unregister (or Client.Close) on disconnect.
func (h *Hub) Register(room string) *Client {
	c := &Client{
		C:    make(chan entity.Message, clientBuffer),
		hub:  h,
		room: room,
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.rooms[room]
	if !ok {
		clients = make(map[*Client]struct{})
		h.rooms[room] = clients
	}

	clients[c] = struct{}{}

	return c
}

// Unregister removes a client and closes its channel. Idempotent.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.rooms[c.room]
	if !ok {
		return
	}

	if _, present := clients[c]; !present {
		return
	}

	delete(clients, c)
	close(c.C)

	if len(clients) == 0 {
		delete(h.rooms, c.room)
	}
}

// Close unregisters the client from its hub. Convenience for the transport's
// defer.
func (c *Client) Close() { c.hub.Unregister(c) }

// Broadcast fans `msg` out to every client in `room`. Delivery is non-blocking:
// if a client's buffer is full the OLDEST queued frame is dropped to make room for
// the newest, so a stalled reader never blocks the hub. Returns the number of
// clients the message reached (queued to).
func (h *Hub) Broadcast(room string, msg entity.Message) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.rooms[room]
	if !ok {
		return 0
	}

	n := 0

	for c := range clients {
		if h.deliver(c, msg) {
			n++
		}
	}

	return n
}

// deliver enqueues msg on c with drop-oldest backpressure. It returns true once
// the message is queued (it always ends up queued unless the channel is closed).
func (h *Hub) deliver(c *Client, msg entity.Message) bool {
	for {
		select {
		case c.C <- msg:
			return true
		default:
			// Buffer full: drop the oldest frame, then retry. The retry loop
			// terminates because draining one slot lets the send succeed; if a
			// concurrent reader also drains, the send still succeeds.
			select {
			case <-c.C:
				h.logger.Debug("client buffer full; dropped oldest frame", "room", c.room)
			default:
			}
		}
	}
}

// RoomSize reports how many clients are subscribed to a room (test/observability).
func (h *Hub) RoomSize(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.rooms[room])
}
