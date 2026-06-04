package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

const tradeID = "trade-123"

func openParams(claimant, respondent uuid.UUID) biz.OpenParams {
	return biz.OpenParams{
		TradeID:     tradeID,
		Claimant:    claimant,
		Respondent:  respondent,
		ReasonCode:  entity.ReasonAuthenticity,
		EvidenceRef: "ipfs://evidence",
	}
}

// TestDispute_Open covers the happy path (creates OPEN + audit + outbox), the
// duplicate-open rejection, bad reason codes, and same-party guard.
func TestDispute_Open(t *testing.T) {
	t.Parallel()

	claimant := uuid.New()
	respondent := uuid.New()

	t.Run("happy path opens dispute and emits dispute.opened", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().
			CreateTx(mock.Anything,
				mock.MatchedBy(func(d entity.Dispute) bool {
					return d.TradeID == tradeID && d.State == entity.StateOpen &&
						d.ClaimantAccountID == claimant && d.RespondentAccountID == respondent &&
						d.ReasonCode == entity.ReasonAuthenticity
				}),
				mock.MatchedBy(func(a entity.DisputeEvent) bool { return a.Action == entity.ActionOpened }),
				mock.MatchedBy(func(o entity.OutboxEvent) bool {
					return o.Subject == biz.SubjectDisputeOpened && o.IdempotencyKey != ""
				}),
			).
			Return(entity.Dispute{ID: uuid.New(), TradeID: tradeID, State: entity.StateOpen}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		d, err := uc.Open(context.Background(), openParams(claimant, respondent))
		require.NoError(t, err)
		require.Equal(t, entity.StateOpen, d.State)
	})

	t.Run("duplicate open rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().CreateTx(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(entity.Dispute{}, biz.ErrResourceExists)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.Open(context.Background(), openParams(claimant, respondent))
		require.ErrorIs(t, err, biz.ErrResourceExists)
	})

	t.Run("invalid reason code rejected before repo", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		uc := biz.NewDispute(discardLogger(), repo)

		p := openParams(claimant, respondent)
		p.ReasonCode = "BOGUS"
		_, err := uc.Open(context.Background(), p)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("claimant equal respondent rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		uc := biz.NewDispute(discardLogger(), repo)

		_, err := uc.Open(context.Background(), openParams(claimant, claimant))
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestDispute_StateWalk walks OPEN -> UNDER_REVIEW -> RESOLVED and asserts illegal
// jumps are rejected by the use case before any write.
func TestDispute_StateWalk(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	admin := uuid.New()

	t.Run("OPEN to UNDER_REVIEW", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByID(mock.Anything, id).
			Return(entity.Dispute{ID: id, State: entity.StateOpen}, nil)
		repo.EXPECT().
			TransitionTx(mock.Anything, id, entity.StateOpen, entity.StateUnderReview,
				mock.MatchedBy(func(a entity.DisputeEvent) bool { return a.Action == entity.ActionReviewStarted })).
			Return(entity.Dispute{ID: id, State: entity.StateUnderReview}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		d, err := uc.StartReview(context.Background(), id, admin)
		require.NoError(t, err)
		require.Equal(t, entity.StateUnderReview, d.State)
	})

	t.Run("review from non-OPEN is illegal", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByID(mock.Anything, id).
			Return(entity.Dispute{ID: id, State: entity.StateUnderReview}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.StartReview(context.Background(), id, admin)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("resolve only from UNDER_REVIEW", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).
			Return(entity.Dispute{ID: id, State: entity.StateOpen}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.Resolve(context.Background(), tradeID, entity.RulingRefundBuyer, admin)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestDispute_Resolve covers each ruling and ruling immutability (a second
// resolve hits the CAS in the repo and returns ErrResourceInvalid).
func TestDispute_Resolve(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	admin := uuid.New()

	rulings := []entity.Ruling{entity.RulingRefundBuyer, entity.RulingReleaseSeller, entity.RulingSplit}
	for _, ruling := range rulings {
		ruling := ruling
		t.Run("resolves with "+string(ruling), func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryDispute(t)
			repo.EXPECT().GetByTrade(mock.Anything, tradeID).
				Return(entity.Dispute{ID: id, TradeID: tradeID, State: entity.StateUnderReview}, nil)
			repo.EXPECT().
				ResolveTx(mock.Anything, id, entity.StateUnderReview, ruling, admin,
					mock.MatchedBy(func(a entity.DisputeEvent) bool { return a.Action == entity.ActionRuled }),
					mock.MatchedBy(func(o entity.OutboxEvent) bool {
						return o.Subject == biz.SubjectDisputeResolved && o.IdempotencyKey != ""
					})).
				Return(entity.Dispute{ID: id, State: entity.StateResolved, Ruling: &ruling}, nil)

			uc := biz.NewDispute(discardLogger(), repo)
			d, err := uc.Resolve(context.Background(), tradeID, ruling, admin)
			require.NoError(t, err)
			require.Equal(t, entity.StateResolved, d.State)
		})
	}

	t.Run("ruling immutable: re-resolve rejected by CAS", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).
			Return(entity.Dispute{ID: id, TradeID: tradeID, State: entity.StateUnderReview}, nil)
		repo.EXPECT().ResolveTx(mock.Anything, id, entity.StateUnderReview, entity.RulingSplit, admin,
			mock.Anything, mock.Anything).
			Return(entity.Dispute{}, biz.ErrResourceInvalid)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.Resolve(context.Background(), tradeID, entity.RulingSplit, admin)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})

	t.Run("invalid ruling rejected before repo", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.Resolve(context.Background(), tradeID, "BOGUS", admin)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestDispute_Withdraw asserts claimant-only + OPEN-only withdrawal.
func TestDispute_Withdraw(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	claimant := uuid.New()
	other := uuid.New()

	t.Run("claimant withdraws an OPEN dispute", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).
			Return(entity.Dispute{ID: id, ClaimantAccountID: claimant, State: entity.StateOpen}, nil)
		repo.EXPECT().
			TransitionTx(mock.Anything, id, entity.StateOpen, entity.StateWithdrawn,
				mock.MatchedBy(func(a entity.DisputeEvent) bool { return a.Action == entity.ActionWithdrawn })).
			Return(entity.Dispute{ID: id, State: entity.StateWithdrawn}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		d, err := uc.Withdraw(context.Background(), tradeID, claimant)
		require.NoError(t, err)
		require.Equal(t, entity.StateWithdrawn, d.State)
	})

	t.Run("non-claimant cannot withdraw", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).
			Return(entity.Dispute{ID: id, ClaimantAccountID: claimant, State: entity.StateOpen}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.Withdraw(context.Background(), tradeID, other)
		require.ErrorIs(t, err, biz.ErrResourceAccessDenied)
	})

	t.Run("cannot withdraw once UNDER_REVIEW", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).
			Return(entity.Dispute{ID: id, ClaimantAccountID: claimant, State: entity.StateUnderReview}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.Withdraw(context.Background(), tradeID, claimant)
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestDispute_Get enforces party-or-admin access and bundles the audit trail.
func TestDispute_Get(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	claimant := uuid.New()
	respondent := uuid.New()
	outsider := uuid.New()

	base := entity.Dispute{ID: id, TradeID: tradeID, ClaimantAccountID: claimant, RespondentAccountID: respondent, State: entity.StateOpen}

	t.Run("claimant reads dispute + trail", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).Return(base, nil)
		repo.EXPECT().ListEvents(mock.Anything, id).
			Return([]entity.DisputeEvent{{ID: uuid.New(), DisputeID: id, Action: entity.ActionOpened}}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		view, err := uc.Get(context.Background(), tradeID, claimant, false)
		require.NoError(t, err)
		require.Len(t, view.Events, 1)
	})

	t.Run("admin reads even when not a party", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).Return(base, nil)
		repo.EXPECT().ListEvents(mock.Anything, id).Return(nil, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.Get(context.Background(), tradeID, outsider, true)
		require.NoError(t, err)
	})

	t.Run("outsider denied", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).Return(base, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.Get(context.Background(), tradeID, outsider, false)
		require.ErrorIs(t, err, biz.ErrResourceAccessDenied)
	})
}

// TestDispute_AddEvidence covers party-only access, terminal rejection, and the
// audit row append.
func TestDispute_AddEvidence(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	claimant := uuid.New()
	respondent := uuid.New()
	outsider := uuid.New()

	t.Run("respondent appends evidence audit row", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).
			Return(entity.Dispute{ID: id, ClaimantAccountID: claimant, RespondentAccountID: respondent, State: entity.StateUnderReview}, nil)
		repo.EXPECT().
			AppendEvent(mock.Anything, mock.MatchedBy(func(a entity.DisputeEvent) bool {
				return a.Action == entity.ActionEvidenceAdded && a.ActorAccountID == respondent && a.DetailRef == "ref-1"
			})).
			Return(nil)

		uc := biz.NewDispute(discardLogger(), repo)
		require.NoError(t, uc.AddEvidence(context.Background(), tradeID, respondent, "ref-1"))
	})

	t.Run("outsider denied", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).
			Return(entity.Dispute{ID: id, ClaimantAccountID: claimant, RespondentAccountID: respondent, State: entity.StateOpen}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		require.ErrorIs(t, uc.AddEvidence(context.Background(), tradeID, outsider, "ref"), biz.ErrResourceAccessDenied)
	})

	t.Run("no evidence on a terminal dispute", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().GetByTrade(mock.Anything, tradeID).
			Return(entity.Dispute{ID: id, ClaimantAccountID: claimant, RespondentAccountID: respondent, State: entity.StateResolved}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		require.ErrorIs(t, uc.AddEvidence(context.Background(), tradeID, claimant, "ref"), biz.ErrResourceInvalid)
	})
}

// TestDispute_ListByState validates the state-filter guard and pass-through.
func TestDispute_ListByState(t *testing.T) {
	t.Parallel()

	t.Run("valid filter passes through", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().ListByState(mock.Anything, entity.StateOpen).
			Return([]entity.Dispute{{ID: uuid.New(), State: entity.StateOpen}}, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		ds, err := uc.ListByState(context.Background(), biz.ListFilter{State: entity.StateOpen})
		require.NoError(t, err)
		require.Len(t, ds, 1)
	})

	t.Run("empty filter lists all", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		repo.EXPECT().ListByState(mock.Anything, entity.State("")).Return(nil, nil)

		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.ListByState(context.Background(), biz.ListFilter{})
		require.NoError(t, err)
	})

	t.Run("unknown filter rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryDispute(t)
		uc := biz.NewDispute(discardLogger(), repo)
		_, err := uc.ListByState(context.Background(), biz.ListFilter{State: "BOGUS"})
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestDispute_RepoErrorPropagates ensures repo errors surface unchanged.
func TestDispute_RepoErrorPropagates(t *testing.T) {
	t.Parallel()

	boom := errors.New("db down")
	repo := mocks.NewMockRepositoryDispute(t)
	repo.EXPECT().GetByTrade(mock.Anything, tradeID).Return(entity.Dispute{}, boom)

	uc := biz.NewDispute(discardLogger(), repo)
	_, err := uc.Withdraw(context.Background(), tradeID, uuid.New())
	require.ErrorIs(t, err, boom)
}
