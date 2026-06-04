package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// envelope wraps a payload in the EventEnvelope wire shape the consumer decodes.
func envelope(t *testing.T, typ, key string, payload any) []byte {
	t.Helper()

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	env, err := json.Marshal(map[string]any{
		"event_id":        uuid.NewString(),
		"idempotency_key": key,
		"producer":        "test",
		"occurred_at":     "2026-06-01T00:00:00Z",
		"type":            typ,
		"version":         1,
		"payload":         json.RawMessage(raw),
	})
	require.NoError(t, err)

	return env
}

// TestConsumer_LockRequested decodes escrow.lock_requested and creates the Dutch
// trade DEPOSIT_LOCKED with a deposit ledger row.
func TestConsumer_LockRequested(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := newUC(repo, nil)
	c := biz.NewEventConsumer(discardLogger(), uc)

	id, lot, buyer, seller := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	raw := envelope(t, biz.SubjectEscrowLockRequested, "k1", map[string]any{
		"trade_id":  id.String(),
		"lot_id":    lot.String(),
		"buyer_id":  buyer.String(),
		"seller_id": seller.String(),
		"state":     string(entity.StateDepositLocked),
		"amount":    map[string]int64{"cents": 10_000},
	})

	require.NoError(t, c.Handle(context.Background(), raw))

	trade, err := repo.GetTrade(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, entity.StateDepositLocked, trade.State)
	require.Equal(t, entity.KindDutch, trade.Kind)
	assertConservation(t, repo, id, false)
}

// TestConsumer_Won decodes auction.won and creates the passive trade UNLOCKED.
func TestConsumer_Won(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := newUC(repo, nil)
	c := biz.NewEventConsumer(discardLogger(), uc)

	id, lot, winner, seller := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	raw := envelope(t, biz.SubjectAuctionWon, "w1", map[string]any{
		"auction_id":    id.String(),
		"lot_id":        lot.String(),
		"winner_id":     winner.String(),
		"seller_id":     seller.String(),
		"cleared_price": map[string]int64{"cents": 50_000},
		"premium":       map[string]int64{"cents": 2_500},
	})

	require.NoError(t, c.Handle(context.Background(), raw))

	trade, err := repo.GetTrade(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, entity.StateUnlocked, trade.State)
	require.Equal(t, entity.KindPassive, trade.Kind)
	require.Equal(t, int64(52_500), trade.ObligationCents())
}

// TestConsumer_Hammer decodes auction.hammer and moves a FULL_LOCKED trade to HELD.
func TestConsumer_Hammer(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := newUC(repo, nil)
	c := biz.NewEventConsumer(discardLogger(), uc)

	id, lot, buyer, seller := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, uc.LockRequested(context.Background(), biz.LockRequest{
		TradeID: id, LotID: lot, BuyerAccountID: buyer, SellerAccountID: seller,
		State: entity.StateFullLocked, AmountCents: 100_000, IdempotencyKey: "lf:" + id.String(),
	}))

	raw := envelope(t, biz.SubjectAuctionHammer, "h1", map[string]any{
		"auction_id":   id.String(),
		"winner_id":    buyer.String(),
		"hammer_price": map[string]int64{"cents": 100_000},
		"premium":      map[string]int64{"cents": 0},
	})

	require.NoError(t, c.Handle(context.Background(), raw))
	require.NoError(t, c.Handle(context.Background(), raw)) // replay deduped

	trade, err := repo.GetTrade(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, entity.StateHeld, trade.State)
	assertConservation(t, repo, id, false)
}

// TestConsumer_DisputeResolved decodes dispute.resolved and applies a ruling.
func TestConsumer_DisputeResolved(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := newUC(repo, nil)
	c := biz.NewEventConsumer(discardLogger(), uc)

	id, winner, _ := seedPassive(t, repo, uc, 100_000, 0)
	_, err := uc.Fund(context.Background(), id, winner, 100_000)
	require.NoError(t, err)

	raw := envelope(t, biz.SubjectDisputeResolved, "d1", map[string]any{
		"dispute_id": uuid.NewString(),
		"trade_id":   id.String(),
		"ruling":     string(entity.RulingReleaseSeller),
	})

	require.NoError(t, c.Handle(context.Background(), raw))

	trade, err := repo.GetTrade(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, entity.StateReleased, trade.State)
	assertConservation(t, repo, id, true)
}

// TestConsumer_UnknownSubjectIgnored asserts unrelated subjects are acked no-ops.
func TestConsumer_UnknownSubjectIgnored(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	uc := newUC(repo, nil)
	c := biz.NewEventConsumer(discardLogger(), uc)

	raw := envelope(t, "kyc.approved", "x1", map[string]any{"account_id": uuid.NewString()})
	require.NoError(t, c.Handle(context.Background(), raw))
}
