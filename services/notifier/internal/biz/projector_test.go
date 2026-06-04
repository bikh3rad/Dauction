package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fixedClock is a deterministic Clock for the projection tests.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

var projNow = time.Date(2026, 6, 1, 12, 0, 30, 0, time.UTC)

// envelope wraps a payload in the EventEnvelope shape the projector decodes.
func envelope(t *testing.T, typ string, payload any) []byte {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	env := map[string]any{
		"event_id":    "evt-1",
		"producer":    "test",
		"type":        typ,
		"version":     1,
		"payload":     json.RawMessage(body),
		"occurred_at": projNow.Format(time.RFC3339Nano),
	}

	raw, err := json.Marshal(env)
	require.NoError(t, err)

	return raw
}

func newProjector() (*biz.Projector, *biz.Hub) {
	hub := biz.NewHub(testLogger())
	reg := biz.NewRegistry()
	p := biz.NewProjector(testLogger(), hub, reg, fixedClock{t: projNow})

	return p, hub
}

// TestProjectionTable drives each consumed event through the projector and asserts
// the resulting broadcast Message shape. It is the proof of the consumed-event →
// broadcast-message projection table, including that a sealed passive bid price is
// NEVER surfaced.
func TestProjectionTable(t *testing.T) {
	t.Parallel()

	openAt := projNow.Add(-30 * time.Second) // 30s elapsed -> some drops

	tests := []struct {
		name    string
		typ     string
		payload any
		room    string
		assert  func(t *testing.T, m entity.Message)
	}{
		{
			name: "auction.opened -> dutch price frame",
			typ:  biz.SubjectAuctionOpened,
			payload: map[string]any{
				"auction_id":         "a1",
				"lot_id":             "lot1",
				"ceiling":            map[string]int64{"cents": 100000},
				"floor":              map[string]int64{"cents": 10000},
				"drop_step":          map[string]int64{"cents": 1000},
				"drop_interval_secs": 10,
				"opened_at":          openAt.Format(time.RFC3339Nano),
			},
			room: biz.AuctionRoom("a1"),
			assert: func(t *testing.T, m entity.Message) {
				require.Equal(t, entity.KindDutchPrice, m.Kind)
				require.Equal(t, "OPEN", m.State)
				require.Equal(t, entity.ModeDutch, m.Mode)
				require.NotNil(t, m.CurrentPriceCents)
				// 30s / 10s = 3 drops of 1000 -> 100000 - 3000 = 97000.
				require.Equal(t, int64(97000), *m.CurrentPriceCents)
				require.NotNil(t, m.NextDropAt)
			},
		},
		{
			name: "auction.hammer -> hammer frame with winner + price",
			typ:  biz.SubjectAuctionHammer,
			payload: map[string]any{
				"auction_id":   "a1",
				"lot_id":       "lot1",
				"winner_id":    "winner-1",
				"hammer_price": map[string]int64{"cents": 95000},
				"premium":      map[string]int64{"cents": 0},
				"hammered_at":  projNow.Format(time.RFC3339Nano),
			},
			room: biz.AuctionRoom("a1"),
			assert: func(t *testing.T, m entity.Message) {
				require.Equal(t, entity.KindHammer, m.Kind)
				require.Equal(t, "HAMMER", m.State)
				require.Equal(t, "winner-1", m.WinnerID)
				require.NotNil(t, m.ClearedCents)
				require.Equal(t, int64(95000), *m.ClearedCents)
			},
		},
		{
			name: "auction.completed -> completed frame carrying final state",
			typ:  biz.SubjectAuctionCompleted,
			payload: map[string]any{
				"auction_id":  "a1",
				"lot_id":      "lot1",
				"final_state": "COMPLETED",
			},
			room: biz.AuctionRoom("a1"),
			assert: func(t *testing.T, m entity.Message) {
				require.Equal(t, entity.KindCompleted, m.Kind)
				require.Equal(t, "COMPLETED", m.State)
			},
		},
		{
			name: "bid.placed -> activity toast ONLY, sealed price never surfaced",
			typ:  biz.SubjectBidPlaced,
			payload: map[string]any{
				"auction_id": "a2",
				"bidder_id":  "bidder-secret",
				"bid_id":     "bid-1",
				"amount":     map[string]int64{"cents": 777777},
				"placed_at":  projNow.Format(time.RFC3339Nano),
			},
			room: biz.AuctionRoom("a2"),
			assert: func(t *testing.T, m entity.Message) {
				require.Equal(t, entity.KindActivity, m.Kind)
				require.Equal(t, 1, m.BidCount)
				// The sealed price and bidder MUST NOT leak anywhere on the frame.
				require.Nil(t, m.ClearedCents)
				require.Nil(t, m.CurrentPriceCents)
				require.Nil(t, m.AmountCents)
				require.Empty(t, m.WinnerID)
				require.Empty(t, m.AccountID)
				blob, _ := json.Marshal(m)
				require.NotContains(t, string(blob), "777777")
				require.NotContains(t, string(blob), "bidder-secret")
			},
		},
		{
			name: "auction.closed -> closing frame",
			typ:  biz.SubjectAuctionClosed,
			payload: map[string]any{
				"auction_id": "a2",
				"lot_id":     "lot2",
				"mode":       entity.ModeVickrey,
				"closed_at":  projNow.Format(time.RFC3339Nano),
			},
			room: biz.AuctionRoom("a2"),
			assert: func(t *testing.T, m entity.Message) {
				require.Equal(t, entity.KindClosed, m.Kind)
				require.Equal(t, "CLOSING", m.State)
				require.Equal(t, entity.ModeVickrey, m.Mode)
			},
		},
		{
			name: "auction.won -> resolved frame reveals cleared price",
			typ:  biz.SubjectAuctionWon,
			payload: map[string]any{
				"auction_id":    "a2",
				"lot_id":        "lot2",
				"winner_id":     "winner-2",
				"cleared_price": map[string]int64{"cents": 42000},
				"premium":       map[string]int64{"cents": 0},
			},
			room: biz.AuctionRoom("a2"),
			assert: func(t *testing.T, m entity.Message) {
				require.Equal(t, entity.KindWon, m.Kind)
				require.Equal(t, "RESOLVED", m.State)
				require.Equal(t, "winner-2", m.WinnerID)
				require.NotNil(t, m.ClearedCents)
				require.Equal(t, int64(42000), *m.ClearedCents)
			},
		},
		{
			name: "escrow.locked -> escrow state to auction room",
			typ:  biz.SubjectEscrowLocked,
			payload: map[string]any{
				"trade_id":       "a3",
				"participant_id": "acct-9",
				"state":          "HELD",
				"amount":         map[string]int64{"cents": 50000},
			},
			room: biz.AuctionRoom("a3"),
			assert: func(t *testing.T, m entity.Message) {
				require.Equal(t, entity.KindEscrowState, m.Kind)
				require.Equal(t, "HELD", m.State)
				require.NotNil(t, m.AmountCents)
				require.Equal(t, int64(50000), *m.AmountCents)
			},
		},
		{
			name: "escrow.refunded -> REFUNDED state to auction room",
			typ:  biz.SubjectEscrowRefunded,
			payload: map[string]any{
				"trade_id":       "a3",
				"participant_id": "acct-9",
				"amount":         map[string]int64{"cents": 10000},
			},
			room: biz.AuctionRoom("a3"),
			assert: func(t *testing.T, m entity.Message) {
				require.Equal(t, entity.KindEscrowState, m.Kind)
				require.Equal(t, "REFUNDED", m.State)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p, hub := newProjector()
			client := hub.Register(tc.room)

			require.NoError(t, p.Handle(context.Background(), envelope(t, tc.typ, tc.payload)))

			select {
			case m := <-client.C:
				tc.assert(t, m)
			default:
				t.Fatal("expected a broadcast frame, got none")
			}
		})
	}
}

// TestEscrowFansToMeRoom asserts an escrow state change also reaches the affected
// participant's me-room (not just the auction room).
func TestEscrowFansToMeRoom(t *testing.T) {
	t.Parallel()

	p, hub := newProjector()
	me := hub.Register(biz.MeRoom("acct-9"))

	raw := envelope(t, biz.SubjectEscrowLocked, map[string]any{
		"trade_id":       "a3",
		"participant_id": "acct-9",
		"state":          "FULL_LOCKED",
		"amount":         map[string]int64{"cents": 50000},
	})
	require.NoError(t, p.Handle(context.Background(), raw))

	m := <-me.C
	require.Equal(t, entity.KindEscrowState, m.Kind)
	require.Equal(t, "acct-9", m.AccountID)
	require.Equal(t, "FULL_LOCKED", m.State)
}

// TestUnknownSubjectIgnored asserts an unrelated subject is a no-op success (acked,
// no broadcast).
func TestUnknownSubjectIgnored(t *testing.T) {
	t.Parallel()

	p, hub := newProjector()
	client := hub.Register(biz.AuctionRoom("a1"))

	raw := envelope(t, "lot.scheduled", map[string]any{"lot_id": "x"})
	require.NoError(t, p.Handle(context.Background(), raw))

	select {
	case <-client.C:
		t.Fatal("unrelated subject must not broadcast")
	default:
	}
}

// TestSnapshotForOpenDutch asserts a reconnecting client's snapshot reflects the
// current computed price for a tracked open Dutch auction.
func TestSnapshotForOpenDutch(t *testing.T) {
	t.Parallel()

	p, _ := newProjector()
	openAt := projNow.Add(-30 * time.Second)

	raw := envelope(t, biz.SubjectAuctionOpened, map[string]any{
		"auction_id":         "a1",
		"lot_id":             "lot1",
		"ceiling":            map[string]int64{"cents": 100000},
		"floor":              map[string]int64{"cents": 10000},
		"drop_step":          map[string]int64{"cents": 1000},
		"drop_interval_secs": 10,
		"opened_at":          openAt.Format(time.RFC3339Nano),
	})
	require.NoError(t, p.Handle(context.Background(), raw))

	snap := p.SnapshotFor("a1")
	require.NotNil(t, snap)
	require.Equal(t, entity.KindSnapshot, snap.Kind)
	require.Equal(t, entity.ModeDutch, snap.Mode)
	require.NotNil(t, snap.CurrentPriceCents)
	require.Equal(t, int64(97000), *snap.CurrentPriceCents)

	// After completion the auction is removed -> no snapshot.
	done := envelope(t, biz.SubjectAuctionCompleted, map[string]any{
		"auction_id": "a1", "lot_id": "lot1", "final_state": "COMPLETED",
	})
	require.NoError(t, p.Handle(context.Background(), done))
	require.Nil(t, p.SnapshotFor("a1"))
}
