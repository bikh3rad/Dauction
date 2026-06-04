package biz

import "application/internal/entity"

// UsecaseLive is the seam the SSE handler depends on: subscribe a client to a
// room, drop it on disconnect, and fetch a connect-time snapshot. The Hub +
// Projector together satisfy it via Subscriber. Keeping it an interface lets the
// handler be unit-tested with a mock and avoids the handler importing concrete
// hub/registry internals.
type UsecaseLive interface {
	// Subscribe registers a client on room and returns it; caller drains C and
	// calls Unsubscribe on disconnect.
	Subscribe(room string) *Client
	// Unsubscribe removes a client and closes its channel.
	Unsubscribe(c *Client)
	// AuctionSnapshot returns the connect-time snapshot for an auction room, or
	// nil if the auction isn't currently tracked.
	AuctionSnapshot(auctionID string) *entity.Message
}

// Subscriber adapts the Hub + Projector into the UsecaseLive seam.
type Subscriber struct {
	hub       *Hub
	projector *Projector
}

var _ UsecaseLive = (*Subscriber)(nil)

// NewSubscriber constructs the live subscription use case.
func NewSubscriber(hub *Hub, projector *Projector) *Subscriber {
	return &Subscriber{hub: hub, projector: projector}
}

// Subscribe registers a client on room.
func (s *Subscriber) Subscribe(room string) *Client { return s.hub.Register(room) }

// Unsubscribe removes a client.
func (s *Subscriber) Unsubscribe(c *Client) { s.hub.Unregister(c) }

// AuctionSnapshot returns the connect-time snapshot for an auction room.
func (s *Subscriber) AuctionSnapshot(auctionID string) *entity.Message {
	return s.projector.SnapshotFor(auctionID)
}

// AuctionRoomKey / MeRoomKey re-export the room key helpers for the handler so it
// doesn't duplicate the keying convention.
func AuctionRoomKey(auctionID string) string { return AuctionRoom(auctionID) }
func MeRoomKey(accountID string) string      { return MeRoom(accountID) }
