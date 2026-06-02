package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestOutboxPublisher_PublishesAndMarks asserts the relay contract: each fetched
// row is published THEN marked published (outbox pattern, at-least-once).
func TestOutboxPublisher_PublishesAndMarks(t *testing.T) {
	t.Parallel()

	ev := entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        biz.SubjectObjectListed,
		IdempotencyKey: "key-1",
		Payload:        []byte(`{"type":"object.listed"}`),
	}

	repo := mocks.NewMockRepositoryOutbox(t)
	pub := mocks.NewMockEventPublisher(t)

	repo.EXPECT().FetchUnpublished(mock.Anything, mock.Anything).Return([]entity.OutboxEvent{ev}, nil).Once()
	pub.EXPECT().Publish(mock.Anything, ev.Subject, ev.Payload).Return(nil).Once()
	repo.EXPECT().MarkPublished(mock.Anything, ev.ID).Return(nil).Once()

	p := biz.NewOutboxPublisher(discardLogger(), repo, pub)
	require.NoError(t, p.DrainOnce(context.Background()))
}

// TestOutboxPublisher_StopsOnPublishError asserts a row is NOT marked published
// when the broker rejects it, so it is retried on the next pass.
func TestOutboxPublisher_StopsOnPublishError(t *testing.T) {
	t.Parallel()

	ev := entity.OutboxEvent{ID: uuid.New(), Subject: biz.SubjectCreditChanged, Payload: []byte(`{}`)}
	boom := errors.New("broker down")

	repo := mocks.NewMockRepositoryOutbox(t)
	pub := mocks.NewMockEventPublisher(t)

	repo.EXPECT().FetchUnpublished(mock.Anything, mock.Anything).Return([]entity.OutboxEvent{ev}, nil).Once()
	pub.EXPECT().Publish(mock.Anything, ev.Subject, ev.Payload).Return(boom).Once()
	// MarkPublished is intentionally NOT expected.

	p := biz.NewOutboxPublisher(discardLogger(), repo, pub)
	require.ErrorIs(t, p.DrainOnce(context.Background()), boom)
}

// TestObjectListedOutbox_Envelope asserts the emitted object.listed envelope
// carries the contract shape: subject, stable idempotency key, producer "vault",
// and the timed-duration enum name (DAYS_5) for a 5-day VICKREY listing.
func TestObjectListedOutbox_Envelope(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	objectID := uuid.New()

	repo := mocks.NewMockRepositoryVault(t)
	repo.EXPECT().GetObject(mock.Anything, objectID).
		Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectInVault, AppraisedValueCents: 620000}, nil)

	var captured entity.OutboxEvent
	repo.EXPECT().
		TransitionTx(mock.Anything, objectID, entity.ObjectInVault, entity.ObjectAppraising, mock.Anything).
		Run(func(_ context.Context, _ uuid.UUID, _ entity.ObjectState, _ entity.ObjectState, o entity.OutboxEvent) {
			captured = o
		}).
		Return(entity.VaultObject{ID: objectID, OwnerAccountID: owner, State: entity.ObjectAppraising}, nil)

	uc := biz.NewVault(discardLogger(), repo)
	_, err := uc.List(context.Background(), owner, objectID, biz.ListRequest{Mode: entity.AuctionVickrey, DurationDays: 5})
	require.NoError(t, err)

	require.Equal(t, biz.SubjectObjectListed, captured.Subject)
	require.NotEmpty(t, captured.IdempotencyKey)
	require.Contains(t, string(captured.Payload), `"producer":"vault"`)
	require.Contains(t, string(captured.Payload), `"DAYS_5"`)
	require.Contains(t, string(captured.Payload), `"VICKREY"`)
}
