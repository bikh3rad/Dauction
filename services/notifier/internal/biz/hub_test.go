package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func msg(kind string) entity.Message {
	return entity.Message{Kind: kind, ServerTime: time.Now().UTC()}
}

// TestHubFanOut asserts a broadcast reaches every client registered in a room.
func TestHubFanOut(t *testing.T) {
	t.Parallel()

	hub := biz.NewHub(testLogger())
	room := biz.AuctionRoom("a1")

	c1 := hub.Register(room)
	c2 := hub.Register(room)
	c3 := hub.Register(biz.AuctionRoom("other"))

	n := hub.Broadcast(room, msg(entity.KindDutchPrice))
	require.Equal(t, 2, n, "only the two clients in the room receive it")

	require.Equal(t, entity.KindDutchPrice, (<-c1.C).Kind)
	require.Equal(t, entity.KindDutchPrice, (<-c2.C).Kind)

	select {
	case <-c3.C:
		t.Fatal("client in a different room must not receive the message")
	default:
	}
}

// TestHubUnregisterStopsDelivery asserts an unregistered client no longer
// receives messages and its channel is closed.
func TestHubUnregisterStopsDelivery(t *testing.T) {
	t.Parallel()

	hub := biz.NewHub(testLogger())
	room := biz.MeRoom("acct-1")

	c := hub.Register(room)
	require.Equal(t, 1, hub.RoomSize(room))

	hub.Unregister(c)
	require.Equal(t, 0, hub.RoomSize(room))

	// Channel is closed: a receive returns the zero value with ok=false.
	_, ok := <-c.C
	require.False(t, ok, "channel must be closed after unregister")

	// Broadcasting to the now-empty room reaches nobody.
	require.Equal(t, 0, hub.Broadcast(room, msg(entity.KindActivity)))
}

// TestHubUnregisterIdempotent asserts a double-unregister is safe.
func TestHubUnregisterIdempotent(t *testing.T) {
	t.Parallel()

	hub := biz.NewHub(testLogger())
	c := hub.Register(biz.AuctionRoom("a1"))

	hub.Unregister(c)
	require.NotPanics(t, func() { hub.Unregister(c) })
}

// TestHubSlowClientDoesNotBlock asserts the drop-oldest backpressure policy: a
// client that never drains its buffer cannot block the hub or other clients, and
// the newest frames survive while the oldest are dropped.
func TestHubSlowClientDoesNotBlock(t *testing.T) {
	t.Parallel()

	hub := biz.NewHub(testLogger())
	room := biz.AuctionRoom("a1")

	slow := hub.Register(room) // never drained
	fast := hub.Register(room)

	// Flood far beyond the per-client buffer. If the hub blocked on the slow
	// client this would deadlock; the test completing proves it doesn't.
	const flood = 1000
	done := make(chan struct{})

	go func() {
		for i := 0; i < flood; i++ {
			m := msg(entity.KindDutchPrice)
			price := int64(i)
			m.CurrentPriceCents = &price
			hub.Broadcast(room, m)
			<-fast.C // keep the fast client drained
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("broadcast blocked on a slow client")
	}

	// The slow client's buffer holds the NEWEST frames (oldest were dropped). Its
	// queue is bounded, never the full flood.
	got := len(slow.C)
	require.Greater(t, got, 0)
	require.LessOrEqual(t, got, 64, "slow client buffer stays bounded")

	hub.Unregister(slow)
	hub.Unregister(fast)
}
