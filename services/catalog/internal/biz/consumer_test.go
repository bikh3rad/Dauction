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

// TestConsumer_ObjectListed_Dutch verifies a DUTCH object.listed maps to a
// CreateFromObjectListed call with no duration and the floor as reserve.
func TestConsumer_ObjectListed_Dutch(t *testing.T) {
	t.Parallel()

	objectID := uuid.New()
	ownerID := uuid.New()
	raw := envelope(t, biz.SubjectObjectListed, "k1", map[string]any{
		"object_id": objectID.String(),
		"owner_id":  ownerID.String(),
		"mode":      "DUTCH",
		"floor":     map[string]any{"cents": 62000000},
	})

	uc := usecasemocks.NewMockUsecaseLot(t)
	uc.EXPECT().
		CreateFromObjectListed(mock.Anything,
			mock.MatchedBy(func(in biz.ObjectListedInput) bool {
				return in.ObjectID == objectID &&
					in.OwnerID == ownerID &&
					in.Mode == entity.ModeDutch &&
					in.DurationDays == nil &&
					in.ReserveCents == 62000000 &&
					in.AppraisedValueCents == 62000000
			}),
			biz.SubjectObjectListed+":k1").
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_ObjectListed_Timed verifies a VICKREY object.listed carries the
// owner-set duration (DAYS_5 -> 5) and an explicit appraised value.
func TestConsumer_ObjectListed_Timed(t *testing.T) {
	t.Parallel()

	objectID := uuid.New()
	ownerID := uuid.New()
	raw := envelope(t, biz.SubjectObjectListed, "k2", map[string]any{
		"object_id": objectID.String(),
		"owner_id":  ownerID.String(),
		"mode":      "VICKREY",
		"duration":  "DAYS_5",
		"floor":     map[string]any{"cents": 1000000},
		"appraised": map[string]any{"cents": 5000000},
	})

	uc := usecasemocks.NewMockUsecaseLot(t)
	uc.EXPECT().
		CreateFromObjectListed(mock.Anything,
			mock.MatchedBy(func(in biz.ObjectListedInput) bool {
				return in.Mode == entity.ModeVickrey &&
					in.DurationDays != nil && *in.DurationDays == 5 &&
					in.ReserveCents == 1000000 &&
					in.AppraisedValueCents == 5000000
			}),
			biz.SubjectObjectListed+":k2").
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

// TestConsumer_ObjectListed_BadMode rejects an unknown auction mode.
func TestConsumer_ObjectListed_BadMode(t *testing.T) {
	t.Parallel()

	raw := envelope(t, biz.SubjectObjectListed, "k3", map[string]any{
		"object_id": uuid.NewString(),
		"owner_id":  uuid.NewString(),
		"mode":      "BLIND",
		"floor":     map[string]any{"cents": 1},
	})

	uc := usecasemocks.NewMockUsecaseLot(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.ErrorIs(t, c.Handle(context.Background(), raw), biz.ErrResourceInvalid)
}

// TestConsumer_IgnoresUnknownSubject acks unrelated subjects without a call.
func TestConsumer_IgnoresUnknownSubject(t *testing.T) {
	t.Parallel()

	raw := envelope(t, "auction.won", "k4", map[string]string{"foo": "bar"})

	uc := usecasemocks.NewMockUsecaseLot(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}
