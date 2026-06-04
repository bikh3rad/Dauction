package handler_test

import (
	"application/internal/biz"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// envelope wraps a payload in the EventEnvelope shape the projector decodes.
func envelope(t *testing.T, typ string, payload any) []byte {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	raw, err := json.Marshal(map[string]any{
		"event_id":    "evt-1",
		"producer":    "test",
		"type":        typ,
		"version":     1,
		"payload":     json.RawMessage(body),
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	return raw
}

func openedEnvelope(t *testing.T, auctionID string, openAt time.Time) []byte {
	t.Helper()

	return envelope(t, biz.SubjectAuctionOpened, map[string]any{
		"auction_id":         auctionID,
		"lot_id":             "lot-" + auctionID,
		"ceiling":            map[string]int64{"cents": 100000},
		"floor":              map[string]int64{"cents": 10000},
		"drop_step":          map[string]int64{"cents": 1000},
		"drop_interval_secs": 10,
		"opened_at":          openAt.Format(time.RFC3339Nano),
	})
}

func hammerEnvelope(t *testing.T, auctionID string) []byte {
	t.Helper()

	return envelope(t, biz.SubjectAuctionHammer, map[string]any{
		"auction_id":   auctionID,
		"lot_id":       "lot-" + auctionID,
		"winner_id":    "w1",
		"hammer_price": map[string]int64{"cents": 95000},
		"premium":      map[string]int64{"cents": 0},
		"hammered_at":  time.Now().UTC().Format(time.RFC3339Nano),
	})
}
