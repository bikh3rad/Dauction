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

// TestConsumer_LotScheduled_Dutch maps a DUTCH lot.scheduled to a
// CreateFromLotScheduled call carrying the lot id + reserve as the floor.
func TestConsumer_LotScheduled_Dutch(t *testing.T) {
	t.Parallel()

	lotID := uuid.New()
	auctionID := uuid.New()
	raw := envelope(t, biz.SubjectLotScheduled, "k1", map[string]any{
		"lot_id":        lotID.String(),
		"auction_id":    auctionID.String(),
		"mode":          "DUTCH",
		"reserve_cents": 62000000,
	})

	uc := usecasemocks.NewMockUsecaseAuction(t)
	uc.EXPECT().
		CreateFromLotScheduled(mock.Anything,
			mock.MatchedBy(func(in biz.LotScheduledInput) bool {
				return in.LotID == lotID &&
					in.AuctionID == auctionID &&
					in.Mode == entity.ModeDutch &&
					in.ReserveCents == 62000000
			}),
			biz.SubjectLotScheduled+":k1").
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_LotScheduled_NonDutch still forwards to the use case (which no-ops
// non-DUTCH modes) so the mode filter lives in one place.
func TestConsumer_LotScheduled_NonDutch(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"VICKREY", "UNIQBID"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			lotID := uuid.New()
			raw := envelope(t, biz.SubjectLotScheduled, "k2", map[string]any{
				"lot_id":        lotID.String(),
				"mode":          mode,
				"reserve_cents": 1000,
			})

			uc := usecasemocks.NewMockUsecaseAuction(t)
			uc.EXPECT().
				CreateFromLotScheduled(mock.Anything,
					mock.MatchedBy(func(in biz.LotScheduledInput) bool {
						return in.Mode == entity.AuctionMode(mode)
					}),
					mock.Anything).
				Return(nil)

			c := biz.NewEventConsumer(discardLogger(), uc)
			require.NoError(t, c.Handle(context.Background(), raw))
		})
	}
}

// TestConsumer_EscrowLocked maps escrow.locked to ApplyEscrowLocked carrying the
// echoed escrow_ref.
func TestConsumer_EscrowLocked(t *testing.T) {
	t.Parallel()

	ref := "auction-dutch:lock:a:b:DEPOSIT_10"
	raw := envelope(t, biz.SubjectEscrowLocked, "k3", map[string]any{
		"trade_id":   "trade-1",
		"state":      "DEPOSIT_LOCKED",
		"amount":     map[string]any{"cents": 100},
		"escrow_ref": ref,
	})

	uc := usecasemocks.NewMockUsecaseAuction(t)
	uc.EXPECT().
		ApplyEscrowLocked(mock.Anything,
			mock.MatchedBy(func(in biz.EscrowLockedInput) bool {
				return in.EscrowRef == ref && in.State == "DEPOSIT_LOCKED" && in.AmountCents == 100
			}),
			biz.SubjectEscrowLocked+":k3").
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_IgnoresUnknownSubject acks unrelated subjects without a call.
func TestConsumer_IgnoresUnknownSubject(t *testing.T) {
	t.Parallel()

	raw := envelope(t, "auction.won", "k4", map[string]string{"foo": "bar"})

	uc := usecasemocks.NewMockUsecaseAuction(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_LotScheduled_BadLotID rejects a malformed lot id.
func TestConsumer_LotScheduled_BadLotID(t *testing.T) {
	t.Parallel()

	raw := envelope(t, biz.SubjectLotScheduled, "k5", map[string]any{
		"lot_id": "not-a-uuid",
		"mode":   "DUTCH",
	})

	uc := usecasemocks.NewMockUsecaseAuction(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.ErrorIs(t, c.Handle(context.Background(), raw), biz.ErrResourceInvalid)
}
