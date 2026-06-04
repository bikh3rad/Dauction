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

// TestOutboxPublisher_DrainOnce verifies each unpublished row is published then
// marked published, in order.
func TestOutboxPublisher_DrainOnce(t *testing.T) {
	t.Parallel()

	e1 := entity.OutboxEvent{ID: uuid.New(), Subject: biz.SubjectBidPlaced, Payload: []byte(`{"a":1}`)}
	e2 := entity.OutboxEvent{ID: uuid.New(), Subject: biz.SubjectAuctionWon, Payload: []byte(`{"b":2}`)}

	repo := mocks.NewMockRepositoryOutbox(t)
	pub := mocks.NewMockEventPublisher(t)

	repo.EXPECT().FetchUnpublished(mock.Anything, mock.Anything).Return([]entity.OutboxEvent{e1, e2}, nil)
	pub.EXPECT().Publish(mock.Anything, e1.Subject, e1.Payload).Return(nil)
	repo.EXPECT().MarkPublished(mock.Anything, e1.ID).Return(nil)
	pub.EXPECT().Publish(mock.Anything, e2.Subject, e2.Payload).Return(nil)
	repo.EXPECT().MarkPublished(mock.Anything, e2.ID).Return(nil)

	p := biz.NewOutboxPublisher(discardLogger(), repo, pub)
	require.NoError(t, p.DrainOnce(context.Background()))
}

// TestOutboxPublisher_StopsOnPublishError verifies a publish failure halts the
// drain so the next pass retries from the same row (no MarkPublished).
func TestOutboxPublisher_StopsOnPublishError(t *testing.T) {
	t.Parallel()

	e1 := entity.OutboxEvent{ID: uuid.New(), Subject: biz.SubjectBidPlaced, Payload: []byte(`{}`)}

	repo := mocks.NewMockRepositoryOutbox(t)
	pub := mocks.NewMockEventPublisher(t)

	repo.EXPECT().FetchUnpublished(mock.Anything, mock.Anything).Return([]entity.OutboxEvent{e1}, nil)
	pub.EXPECT().Publish(mock.Anything, e1.Subject, e1.Payload).Return(errors.New("broker down"))

	p := biz.NewOutboxPublisher(discardLogger(), repo, pub)
	require.Error(t, p.DrainOnce(context.Background()))
}
