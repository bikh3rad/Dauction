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

func TestConsumer_KycApproved(t *testing.T) {
	t.Parallel()

	// kyc.approved is now the sole membership trigger (invites removed): the
	// consumer mirrors the KYC status AND elevates GUEST->MEMBER, each with its
	// own scoped inbox key.
	accountID := uuid.New()
	raw := envelope(t, biz.SubjectKycApproved, "k2", map[string]string{
		"account_id":    accountID.String(),
		"submission_id": uuid.NewString(),
	})

	uc := mocks.NewMockUsecaseAccount(t)
	uc.EXPECT().
		ApproveKyc(mock.Anything, accountID, biz.SubjectKycApproved+":k2").
		Return(nil)
	uc.EXPECT().
		ElevateToMember(mock.Anything, accountID, biz.SubjectKycApproved+":member:k2").
		Return(nil)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

func TestConsumer_IgnoresUnknownSubject(t *testing.T) {
	t.Parallel()

	raw := envelope(t, "auction.won", "k3", map[string]string{"foo": "bar"})

	// No usecase calls expected; mock asserts no unexpected calls on cleanup.
	uc := mocks.NewMockUsecaseAccount(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.NoError(t, c.Handle(context.Background(), raw))
}

func TestConsumer_BadAccountID(t *testing.T) {
	t.Parallel()

	raw := envelope(t, biz.SubjectKycApproved, "k4", map[string]string{
		"account_id": "not-a-uuid",
	})

	uc := mocks.NewMockUsecaseAccount(t)

	c := biz.NewEventConsumer(discardLogger(), uc)
	require.ErrorIs(t, c.Handle(context.Background(), raw), biz.ErrResourceInvalid)
}
