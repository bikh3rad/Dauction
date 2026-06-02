package biz_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/mocks"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func draftLot(id uuid.UUID, mode entity.AuctionMode) entity.Lot {
	return entity.Lot{
		ID:       id,
		ObjectID: uuid.New(),
		Mode:     mode,
		State:    entity.LotDraft,
		ISOWeek:  "2026-W23",
	}
}

// TestLot_Certify covers the certification gate: a lot may only move
// DRAFT->CERTIFIED, and only when it carries a PASS attestation.
func TestLot_Certify(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	passAtt := entity.Attestation{ID: uuid.New(), LotID: id, Result: entity.AttestPass}

	tests := []struct {
		name      string
		state     entity.LotState
		atts      []entity.Attestation
		wantErr   error
		wantWrite bool
	}{
		{
			name:      "draft with PASS certifies",
			state:     entity.LotDraft,
			atts:      []entity.Attestation{passAtt},
			wantWrite: true,
		},
		{
			name:    "draft without PASS is gated",
			state:   entity.LotDraft,
			atts:    []entity.Attestation{{Result: entity.AttestFail}},
			wantErr: biz.ErrResourceInvalid,
		},
		{
			name:    "draft with no attestation is gated",
			state:   entity.LotDraft,
			atts:    nil,
			wantErr: biz.ErrResourceInvalid,
		},
		{
			name:    "already certified is illegal",
			state:   entity.LotCertified,
			wantErr: biz.ErrResourceInvalid,
		},
		{
			name:    "scheduled cannot re-certify",
			state:   entity.LotScheduled,
			wantErr: biz.ErrResourceInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryLot(t)
			l := draftLot(id, entity.ModeDutch)
			l.State = tc.state
			repo.EXPECT().Get(mock.Anything, id).Return(l, nil)

			if tc.state == entity.LotDraft {
				repo.EXPECT().AttestationsByLot(mock.Anything, id).Return(tc.atts, nil)
			}

			if tc.wantWrite {
				certified := l
				certified.State = entity.LotCertified
				repo.EXPECT().
					CertifyTx(mock.Anything, id, mock.MatchedBy(func(o entity.OutboxEvent) bool {
						return o.Subject == biz.SubjectLotCertified && o.IdempotencyKey != ""
					})).
					Return(certified, nil)
			}

			uc := biz.NewLot(discardLogger(), repo)
			got, err := uc.Certify(context.Background(), id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, entity.LotCertified, got.State)
		})
	}
}

// TestLot_Schedule_StateGate covers the state machine: only a CERTIFIED lot may
// be scheduled. The weekly cap itself is enforced atomically in the repo (see
// TestLot_Schedule_WeeklyCap for that boundary via the repo seam).
func TestLot_Schedule_StateGate(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name      string
		state     entity.LotState
		wantErr   error
		wantWrite bool
	}{
		{name: "certified schedules", state: entity.LotCertified, wantWrite: true},
		{name: "draft cannot schedule", state: entity.LotDraft, wantErr: biz.ErrResourceInvalid},
		{name: "already scheduled is illegal", state: entity.LotScheduled, wantErr: biz.ErrResourceInvalid},
		{name: "rejected cannot schedule", state: entity.LotRejected, wantErr: biz.ErrResourceInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryLot(t)
			l := draftLot(id, entity.ModeDutch)
			l.State = tc.state
			repo.EXPECT().Get(mock.Anything, id).Return(l, nil)

			if tc.wantWrite {
				scheduled := l
				scheduled.State = entity.LotScheduled
				repo.EXPECT().
					ScheduleTx(mock.Anything, id, mock.Anything, biz.WeeklySupplyCap, mock.MatchedBy(func(o entity.OutboxEvent) bool {
						return o.Subject == biz.SubjectLotScheduled && o.IdempotencyKey != ""
					})).
					Return(scheduled, nil)
			}

			uc := biz.NewLot(discardLogger(), repo)
			got, err := uc.Schedule(context.Background(), id, time.Now())

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, entity.LotScheduled, got.State)
		})
	}
}

// TestLot_Schedule_WeeklyCap asserts the cap boundary: the use case passes
// WeeklySupplyCap (32) to the repo, the 32nd lot succeeds, and when the repo's
// atomic cap check rejects the 33rd it surfaces ErrResourceInvalid.
func TestLot_Schedule_WeeklyCap(t *testing.T) {
	t.Parallel()

	require.Equal(t, 32, biz.WeeklySupplyCap)

	id := uuid.New()
	l := draftLot(id, entity.ModeDutch)
	l.State = entity.LotCertified

	t.Run("32nd within cap succeeds", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryLot(t)
		repo.EXPECT().Get(mock.Anything, id).Return(l, nil)
		scheduled := l
		scheduled.State = entity.LotScheduled
		repo.EXPECT().
			ScheduleTx(mock.Anything, id, mock.Anything, 32, mock.Anything).
			Return(scheduled, nil)

		uc := biz.NewLot(discardLogger(), repo)
		_, err := uc.Schedule(context.Background(), id, time.Now())
		require.NoError(t, err)
	})

	t.Run("33rd over cap rejected", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryLot(t)
		repo.EXPECT().Get(mock.Anything, id).Return(l, nil)
		// Repo's conditional update enforces the cap and returns invalid.
		repo.EXPECT().
			ScheduleTx(mock.Anything, id, mock.Anything, 32, mock.Anything).
			Return(entity.Lot{}, biz.ErrResourceInvalid)

		uc := biz.NewLot(discardLogger(), repo)
		_, err := uc.Schedule(context.Background(), id, time.Now())
		require.ErrorIs(t, err, biz.ErrResourceInvalid)
	})
}

// TestLot_CreateFromObjectListed covers object.listed consumption: a fresh event
// creates a DRAFT lot; a duplicate (inbox key already seen / object already has a
// lot) is a no-op success so a replay never creates a second lot.
func TestLot_CreateFromObjectListed(t *testing.T) {
	t.Parallel()

	objectID := uuid.New()
	ownerID := uuid.New()
	days := int32(5)

	base := biz.ObjectListedInput{
		ObjectID:            objectID,
		OwnerID:             ownerID,
		Mode:                entity.ModeVickrey,
		DurationDays:        &days,
		ReserveCents:        1000,
		AppraisedValueCents: 5000,
		ISOWeek:             "2026-W23",
	}

	t.Run("fresh event creates a draft lot", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryLot(t)
		repo.EXPECT().
			CreateLotTx(mock.Anything,
				mock.MatchedBy(func(l entity.Lot) bool {
					return l.ObjectID == objectID &&
						l.SellerAccountID == ownerID &&
						l.Mode == entity.ModeVickrey &&
						l.DurationDays != nil && *l.DurationDays == 5 &&
						l.State == entity.LotDraft &&
						l.ISOWeek == "2026-W23"
				}),
				"obj-key").
			Return(nil)

		uc := biz.NewLot(discardLogger(), repo)
		require.NoError(t, uc.CreateFromObjectListed(context.Background(), base, "obj-key"))
	})

	t.Run("duplicate event is a no-op success", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryLot(t)
		repo.EXPECT().
			CreateLotTx(mock.Anything, mock.Anything, "obj-key").
			Return(biz.ErrResourceExists)

		uc := biz.NewLot(discardLogger(), repo)
		require.NoError(t, uc.CreateFromObjectListed(context.Background(), base, "obj-key"))
	})

	t.Run("timed mode without duration is invalid", func(t *testing.T) {
		t.Parallel()

		repo := mocks.NewMockRepositoryLot(t)
		in := base
		in.DurationDays = nil

		uc := biz.NewLot(discardLogger(), repo)
		require.ErrorIs(t,
			uc.CreateFromObjectListed(context.Background(), in, "obj-key"),
			biz.ErrResourceInvalid)
	})
}

// TestLot_Attest covers attestation recording and the FAIL-on-DRAFT rejection.
func TestLot_Attest(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	inspector := uuid.New()

	tests := []struct {
		name       string
		state      entity.LotState
		result     entity.AttestationResult
		wantErr    error
		wantReject bool
	}{
		{name: "pass on draft", state: entity.LotDraft, result: entity.AttestPass, wantReject: false},
		{name: "fail on draft rejects lot", state: entity.LotDraft, result: entity.AttestFail, wantReject: true},
		{name: "pass on certified", state: entity.LotCertified, result: entity.AttestPass, wantReject: false},
		{name: "cannot attest scheduled", state: entity.LotScheduled, result: entity.AttestPass, wantErr: biz.ErrResourceInvalid},
		{name: "cannot attest rejected", state: entity.LotRejected, result: entity.AttestPass, wantErr: biz.ErrResourceInvalid},
		{name: "invalid result", state: entity.LotDraft, result: "MAYBE", wantErr: biz.ErrResourceInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := mocks.NewMockRepositoryLot(t)
			l := draftLot(id, entity.ModeDutch)
			l.State = tc.state

			// Get is only consulted for valid results.
			if tc.result.Valid() {
				repo.EXPECT().Get(mock.Anything, id).Return(l, nil)
			}

			if tc.wantErr == nil {
				repo.EXPECT().
					RecordAttestationTx(mock.Anything,
						mock.MatchedBy(func(a entity.Attestation) bool {
							return a.LotID == id && a.Result == tc.result
						}),
						tc.wantReject,
						mock.MatchedBy(func(o entity.OutboxEvent) bool {
							return o.Subject == biz.SubjectAttestationRecorded && o.IdempotencyKey != ""
						})).
					Return(nil)
			}

			uc := biz.NewLot(discardLogger(), repo)
			_, err := uc.Attest(context.Background(), id, biz.AttestInput{
				InspectorID: inspector,
				Result:      tc.result,
				NotesRef:    "ref",
			})

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
