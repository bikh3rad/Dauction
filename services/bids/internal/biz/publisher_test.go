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
		Subject:        biz.SubjectBidsDebited,
		IdempotencyKey: "bid-1",
		Payload:        []byte(`{"type":"bids.debited"}`),
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

	ev := entity.OutboxEvent{ID: uuid.New(), Subject: biz.SubjectBidsPurchased, Payload: []byte(`{}`)}
	boom := errors.New("broker down")

	repo := mocks.NewMockRepositoryOutbox(t)
	pub := mocks.NewMockEventPublisher(t)

	repo.EXPECT().FetchUnpublished(mock.Anything, mock.Anything).Return([]entity.OutboxEvent{ev}, nil).Once()
	pub.EXPECT().Publish(mock.Anything, ev.Subject, ev.Payload).Return(boom).Once()
	// MarkPublished is intentionally NOT expected.

	p := biz.NewOutboxPublisher(discardLogger(), repo, pub)
	require.ErrorIs(t, p.DrainOnce(context.Background()), boom)
}

// TestNewDebitedOutbox asserts the bids.debited outbox row carries the right
// subject, the idempotency_key matching the bid write, and a well-formed envelope.
func TestNewDebitedOutbox(t *testing.T) {
	t.Parallel()

	account := uuid.New()
	ob, err := biz.NewDebitedOutbox(account, 1, 41, "bid-77")

	require.NoError(t, err)
	require.Equal(t, biz.SubjectBidsDebited, ob.Subject)
	require.Equal(t, "bid-77", ob.IdempotencyKey)
	require.NotEmpty(t, ob.Payload)
	require.Contains(t, string(ob.Payload), "bids.debited")
	require.Contains(t, string(ob.Payload), "bid-77")
}
