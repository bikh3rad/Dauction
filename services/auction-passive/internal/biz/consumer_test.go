package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	usecasemocks "application/internal/mocks/usecase"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// envelope is a minimal builder mirroring the events.v1 EventEnvelope wire shape.
func envelope(t *testing.T, subject, key string, payload any) []byte {
	t.Helper()

	p, err := json.Marshal(payload)
	require.NoError(t, err)

	raw, err := json.Marshal(map[string]any{
		"event_id":        uuid.NewString(),
		"idempotency_key": key,
		"producer":        "test",
		"type":            subject,
		"version":         1,
		"payload":         json.RawMessage(p),
	})
	require.NoError(t, err)

	return raw
}

// TestConsumer_LotScheduled_Vickrey verifies a VICKREY lot.scheduled maps to a
// CreateFromLotScheduled call with the right mode + duration + scoped inbox key.
func TestConsumer_LotScheduled_Vickrey(t *testing.T) {
	t.Parallel()

	lotID := uuid.New()
	raw := envelope(t, biz.SubjectLotScheduled, "k1", map[string]any{
		"lot_id":        lotID.String(),
		"object_id":     uuid.NewString(),
		"mode":          "VICKREY",
		"duration_days": 5,
		"scheduled_at":  "2026-06-01T00:00:00Z",
		"reserve_cents": 1000000,
		"week":          "2026-W23",
	})

	uc := usecasemocks.NewMockUsecaseAuction(t)
	uc.EXPECT().
		CreateFromLotScheduled(mock.Anything,
			mock.MatchedBy(func(in biz.LotScheduledInput) bool {
				return in.LotID == lotID &&
					in.Mode == entity.ModeVickrey &&
					in.DurationDays == 5 &&
					in.ReserveCents == 1000000
			}),
			biz.SubjectLotScheduled+":k1").
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_LotScheduled_UniqBid verifies a UNIQBID lot.scheduled is created.
func TestConsumer_LotScheduled_UniqBid(t *testing.T) {
	t.Parallel()

	lotID := uuid.New()
	raw := envelope(t, biz.SubjectLotScheduled, "k2", map[string]any{
		"lot_id":        lotID.String(),
		"mode":          "UNIQBID",
		"duration_days": 7,
		"scheduled_at":  "2026-06-01T00:00:00Z",
		"reserve_cents": 500,
		"week":          "2026-W23",
	})

	uc := usecasemocks.NewMockUsecaseAuction(t)
	uc.EXPECT().
		CreateFromLotScheduled(mock.Anything,
			mock.MatchedBy(func(in biz.LotScheduledInput) bool {
				return in.Mode == entity.ModeUniqBid && in.DurationDays == 7
			}),
			biz.SubjectLotScheduled+":k2").
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_LotScheduled_DutchIgnored verifies a DUTCH lot.scheduled is acked
// and NOT turned into a passive auction (auction-dutch owns it).
func TestConsumer_LotScheduled_DutchIgnored(t *testing.T) {
	t.Parallel()

	raw := envelope(t, biz.SubjectLotScheduled, "k3", map[string]any{
		"lot_id":        uuid.NewString(),
		"mode":          "DUTCH",
		"duration_days": 0,
		"scheduled_at":  "2026-06-01T00:00:00Z",
	})

	uc := usecasemocks.NewMockUsecaseAuction(t) // no CreateFromLotScheduled expected

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_BidsDebited_Reconciliation verifies bids.debited is acked without
// mutating state (no use-case call).
func TestConsumer_BidsDebited_Reconciliation(t *testing.T) {
	t.Parallel()

	raw := envelope(t, biz.SubjectBidsDebited, "k4", map[string]any{
		"account_id":      uuid.NewString(),
		"amount":          map[string]any{"credits": 1},
		"idempotency_key": "bid:abc",
		"balance":         map[string]any{"credits": 9},
	})

	uc := usecasemocks.NewMockUsecaseAuction(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_IgnoresUnknownSubject acks unrelated subjects without a call.
func TestConsumer_IgnoresUnknownSubject(t *testing.T) {
	t.Parallel()

	raw := envelope(t, "escrow.locked", "k5", map[string]string{"foo": "bar"})

	uc := usecasemocks.NewMockUsecaseAuction(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}
