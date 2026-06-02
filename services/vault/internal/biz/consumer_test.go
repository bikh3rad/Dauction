package biz_test

import (
	"application/internal/biz"
	"application/internal/mocks"
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

func TestConsumer_AuctionCompleted_VaultCredit(t *testing.T) {
	t.Parallel()

	objectID := uuid.New()
	raw := envelope(t, biz.SubjectAuctionCompleted, "k1", map[string]any{
		"auction_id":      uuid.NewString(),
		"lot_id":          uuid.NewString(),
		"object_id":       objectID.String(),
		"final_state":     "COMPLETED",
		"as_vault_credit": true,
		"release":         map[string]int64{"cents": 11000},
	})

	uc := mocks.NewMockUsecaseVault(t)
	uc.EXPECT().
		SettleAuctionCompleted(mock.Anything, mock.MatchedBy(func(in biz.AuctionCompletedInput) bool {
			return in.ObjectID == objectID &&
				in.AsVaultCredit &&
				in.ReleaseCents == 11000 &&
				in.IdempotencyKey == biz.SubjectAuctionCompleted+":k1"
		})).
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

func TestConsumer_AuctionCompleted_CashRelease(t *testing.T) {
	t.Parallel()

	objectID := uuid.New()
	raw := envelope(t, biz.SubjectAuctionCompleted, "k2", map[string]any{
		"object_id":   objectID.String(),
		"final_state": "COMPLETED",
	})

	uc := mocks.NewMockUsecaseVault(t)
	uc.EXPECT().
		SettleAuctionCompleted(mock.Anything, mock.MatchedBy(func(in biz.AuctionCompletedInput) bool {
			return in.ObjectID == objectID && !in.AsVaultCredit && in.ReleaseCents == 0
		})).
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

func TestConsumer_AuctionCompleted_NonCompletedIgnored(t *testing.T) {
	t.Parallel()

	raw := envelope(t, biz.SubjectAuctionCompleted, "k3", map[string]any{
		"object_id":   uuid.NewString(),
		"final_state": "ABORTED",
	})

	// No usecase call expected for a non-COMPLETED final state.
	uc := mocks.NewMockUsecaseVault(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

func TestConsumer_IgnoresUnknownSubject(t *testing.T) {
	t.Parallel()

	raw := envelope(t, "escrow.released", "k4", map[string]string{"foo": "bar"})

	uc := mocks.NewMockUsecaseVault(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

func TestConsumer_BadObjectID(t *testing.T) {
	t.Parallel()

	raw := envelope(t, biz.SubjectAuctionCompleted, "k5", map[string]any{
		"object_id":   "not-a-uuid",
		"final_state": "COMPLETED",
	})

	uc := mocks.NewMockUsecaseVault(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.ErrorIs(t, c.Handle(context.Background(), raw), biz.ErrResourceInvalid)
}
