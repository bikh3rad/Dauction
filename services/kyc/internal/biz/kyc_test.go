package biz

import (
	"application/internal/entity"
	"application/internal/mocks"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fixedClock returns a deterministic time and a fixed UUID generator so we can
// assert payloads and expiry boundaries exactly.
var (
	baseTime = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	accID    = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	subID    = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	chID     = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	eventID  = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	adminID  = uuid.MustParse("55555555-5555-5555-5555-555555555555")
)

func newUC(t *testing.T, cfg Config) (*kyc, *mocks.MockRepositoryKyc, *mocks.MockEventPublisher) {
	t.Helper()

	repo := mocks.NewMockRepositoryKyc(t)
	pub := mocks.NewMockEventPublisher(t)

	uc := NewKyc(slog.Default(), repo, pub, cfg)
	uc.now = func() time.Time { return baseTime }

	// Deterministic IDs: first call -> subID, second -> chID, then eventID.
	ids := []uuid.UUID{subID, chID, eventID}
	i := 0
	uc.newID = func() uuid.UUID {
		id := ids[i%len(ids)]
		i++
		return id
	}

	return uc, repo, pub
}

func TestStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		params    StartParams
		latest    *entity.Submission
		latestErr error
		wantErr   error
		expectGen bool
	}{
		{
			name:      "first submission succeeds",
			params:    StartParams{AccountID: accID, DocType: entity.DocEmiratesID, DocRef: "ref-1", Phone: "+971500000000"},
			latestErr: ErrResourceNotFound,
			expectGen: true,
		},
		{
			name:      "resubmit after rejection succeeds",
			params:    StartParams{AccountID: accID, DocType: entity.DocPassport, DocRef: "ref-2", Phone: "+971500000000"},
			latest:    &entity.Submission{State: entity.SubmissionRejected},
			expectGen: true,
		},
		{
			name:    "already approved is rejected",
			params:  StartParams{AccountID: accID, DocType: entity.DocEmiratesID, DocRef: "ref", Phone: "+971500000000"},
			latest:  &entity.Submission{State: entity.SubmissionApproved},
			wantErr: ErrResourceExists,
		},
		{
			name:    "in-progress submission is rejected",
			params:  StartParams{AccountID: accID, DocType: entity.DocEmiratesID, DocRef: "ref", Phone: "+971500000000"},
			latest:  &entity.Submission{State: entity.SubmissionStarted},
			wantErr: ErrResourceExists,
		},
		{
			name:    "invalid doc type",
			params:  StartParams{AccountID: accID, DocType: "DRIVERS_LICENSE", DocRef: "ref", Phone: "+971500000000"},
			wantErr: ErrResourceInvalid,
		},
		{
			name:    "missing phone",
			params:  StartParams{AccountID: accID, DocType: entity.DocEmiratesID, DocRef: "ref"},
			wantErr: ErrResourceInvalid,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, _ := newUC(t, Config{})

			if tc.wantErr == nil || errors.Is(tc.wantErr, ErrResourceExists) {
				if tc.latest != nil {
					repo.EXPECT().GetLatestSubmission(mock.Anything, accID).Return(*tc.latest, nil)
				} else if tc.latestErr != nil {
					repo.EXPECT().GetLatestSubmission(mock.Anything, accID).Return(entity.Submission{}, tc.latestErr)
				}
			}

			if tc.expectGen {
				repo.EXPECT().
					CreateSubmission(mock.Anything, mock.MatchedBy(func(s entity.Submission) bool {
						return s.State == entity.SubmissionStarted && s.ID == subID
					}), mock.MatchedBy(func(c entity.OTPChallenge) bool {
						return c.CodeHash != "" && c.ExpiresAt.Equal(baseTime.Add(DefaultOTPTTL))
					})).
					Return(nil)
			}

			res, err := uc.Start(context.Background(), tc.params)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, subID, res.SubmissionID)
			require.Equal(t, chID, res.ChallengeID)
			// dev code present (Production=false)
			require.Len(t, res.DevCode, 6)
		})
	}
}

func TestVerify(t *testing.T) {
	t.Parallel()

	const code = "123456"
	goodHash := hashOTP(code)

	openChallenge := func() entity.OTPChallenge {
		return entity.OTPChallenge{
			ID:        chID,
			CodeHash:  goodHash,
			ExpiresAt: baseTime.Add(5 * time.Minute),
		}
	}

	tests := []struct {
		name        string
		sub         entity.Submission
		subErr      error
		challenge   *entity.OTPChallenge
		input       string
		expectBump  bool
		expectVerify bool
		wantErr     error
		wantState   entity.SubmissionState
	}{
		{
			name:         "happy path advances to SUBMITTED",
			sub:          entity.Submission{ID: subID, State: entity.SubmissionStarted},
			challenge:    ptr(openChallenge()),
			input:        code,
			expectVerify: true,
			wantState:    entity.SubmissionSubmitted,
		},
		{
			name:    "submission not found",
			subErr:  ErrResourceNotFound,
			input:   code,
			wantErr: ErrResourceNotFound,
		},
		{
			name:    "already submitted cannot re-verify",
			sub:     entity.Submission{ID: subID, State: entity.SubmissionSubmitted},
			input:   code,
			wantErr: ErrResourceInvalid,
		},
		{
			name: "expired otp",
			sub:  entity.Submission{ID: subID, State: entity.SubmissionStarted},
			challenge: func() *entity.OTPChallenge {
				c := openChallenge()
				c.ExpiresAt = baseTime.Add(-time.Second)
				return &c
			}(),
			input:   code,
			wantErr: ErrResourceInvalid,
		},
		{
			name: "attempts exceeded",
			sub:  entity.Submission{ID: subID, State: entity.SubmissionStarted},
			challenge: func() *entity.OTPChallenge {
				c := openChallenge()
				c.Attempts = DefaultMaxAttempts
				return &c
			}(),
			input:   code,
			wantErr: ErrResourceInvalid,
		},
		{
			name:       "wrong code bumps attempts",
			sub:        entity.Submission{ID: subID, State: entity.SubmissionStarted},
			challenge:  ptr(openChallenge()),
			input:      "000000",
			expectBump: true,
			wantErr:    ErrResourceInvalid,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, _ := newUC(t, Config{})

			if tc.subErr != nil {
				repo.EXPECT().GetLatestSubmission(mock.Anything, accID).Return(entity.Submission{}, tc.subErr)
			} else {
				repo.EXPECT().GetLatestSubmission(mock.Anything, accID).Return(tc.sub, nil)
			}

			if tc.challenge != nil {
				repo.EXPECT().GetOpenChallenge(mock.Anything, subID).Return(*tc.challenge, nil)
			}

			if tc.expectBump {
				repo.EXPECT().IncrementChallengeAttempts(mock.Anything, chID).Return(nil)
			}

			if tc.expectVerify {
				repo.EXPECT().MarkVerifiedAndSubmit(mock.Anything, subID, chID, baseTime).Return(nil)
			}

			got, err := uc.Verify(context.Background(), accID, tc.input)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantState, got.State)
		})
	}
}

func TestDecide(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		current     entity.Submission
		getErr      error
		approve     bool
		reason      string
		wantErr     error
		wantState   entity.SubmissionState
		wantSubject string
	}{
		{
			name:        "approve submitted",
			current:     entity.Submission{ID: subID, AccountID: accID, State: entity.SubmissionSubmitted},
			approve:     true,
			wantState:   entity.SubmissionApproved,
			wantSubject: subjectKycApproved,
		},
		{
			name:        "reject submitted with reason",
			current:     entity.Submission{ID: subID, AccountID: accID, State: entity.SubmissionSubmitted},
			reason:      "KYC_REJECTED",
			wantState:   entity.SubmissionRejected,
			wantSubject: subjectKycRejected,
		},
		{
			name:    "double-approve already approved is illegal",
			current: entity.Submission{ID: subID, AccountID: accID, State: entity.SubmissionApproved},
			approve: true,
			wantErr: ErrResourceInvalid,
		},
		{
			name:    "approve a rejected submission is illegal",
			current: entity.Submission{ID: subID, AccountID: accID, State: entity.SubmissionRejected},
			approve: true,
			wantErr: ErrResourceInvalid,
		},
		{
			name:    "submission not found",
			getErr:  ErrResourceNotFound,
			approve: true,
			wantErr: ErrResourceNotFound,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, _ := newUC(t, Config{})

			if tc.getErr != nil {
				repo.EXPECT().GetSubmission(mock.Anything, subID).Return(entity.Submission{}, tc.getErr)
			} else {
				repo.EXPECT().GetSubmission(mock.Anything, subID).Return(tc.current, nil)
			}

			if tc.wantErr == nil {
				repo.EXPECT().
					DecideSubmission(mock.Anything,
						mock.MatchedBy(func(s entity.Submission) bool {
							return s.State == tc.wantState && s.DecidedBy != nil && *s.DecidedBy == adminID
						}),
						mock.MatchedBy(func(o entity.OutboxEvent) bool {
							return o.Subject == tc.wantSubject &&
								o.IdempotencyKey == subID.String()+":"+string(tc.wantState)
						})).
					Return(nil)
			}

			var (
				got entity.Submission
				err error
			)

			if tc.approve {
				got, err = uc.Approve(context.Background(), subID, adminID)
			} else {
				got, err = uc.Reject(context.Background(), RejectParams{SubmissionID: subID, DecidedBy: adminID, Reason: tc.reason})
			}

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantState, got.State)
		})
	}
}

func TestRejectRequiresReason(t *testing.T) {
	t.Parallel()

	uc, _, _ := newUC(t, Config{})

	_, err := uc.Reject(context.Background(), RejectParams{SubmissionID: subID, DecidedBy: adminID, Reason: ""})
	require.ErrorIs(t, err, ErrResourceInvalid)
}

// TestFullLifecycle walks STARTED -> SUBMITTED -> APPROVED, asserting the emitted
// outbox payload matches the proto EventEnvelope shape.
func TestFullLifecycle(t *testing.T) {
	t.Parallel()

	uc, repo, _ := newUC(t, Config{})

	// Start
	repo.EXPECT().GetLatestSubmission(mock.Anything, accID).Return(entity.Submission{}, ErrResourceNotFound).Once()
	repo.EXPECT().CreateSubmission(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	start, err := uc.Start(context.Background(), StartParams{
		AccountID: accID, DocType: entity.DocEmiratesID, DocRef: "ref", Phone: "+971500000000",
	})
	require.NoError(t, err)
	require.NotEmpty(t, start.DevCode)

	// Verify
	repo.EXPECT().GetLatestSubmission(mock.Anything, accID).
		Return(entity.Submission{ID: subID, AccountID: accID, State: entity.SubmissionStarted}, nil).Once()
	repo.EXPECT().GetOpenChallenge(mock.Anything, subID).
		Return(entity.OTPChallenge{ID: chID, CodeHash: hashOTP(start.DevCode), ExpiresAt: baseTime.Add(time.Minute)}, nil).Once()
	repo.EXPECT().MarkVerifiedAndSubmit(mock.Anything, subID, chID, baseTime).Return(nil).Once()

	sub, err := uc.Verify(context.Background(), accID, start.DevCode)
	require.NoError(t, err)
	require.Equal(t, entity.SubmissionSubmitted, sub.State)

	// Approve, capturing the outbox payload
	var captured entity.OutboxEvent
	repo.EXPECT().GetSubmission(mock.Anything, subID).
		Return(entity.Submission{ID: subID, AccountID: accID, State: entity.SubmissionSubmitted}, nil).Once()
	repo.EXPECT().DecideSubmission(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ entity.Submission, o entity.OutboxEvent) { captured = o }).
		Return(nil).Once()

	approved, err := uc.Approve(context.Background(), subID, adminID)
	require.NoError(t, err)
	require.Equal(t, entity.SubmissionApproved, approved.State)

	require.Equal(t, subjectKycApproved, captured.Subject)

	var env eventEnvelope
	require.NoError(t, json.Unmarshal(captured.Payload, &env))
	require.Equal(t, "kyc", env.Producer)
	require.Equal(t, subjectKycApproved, env.Type)
	require.NotNil(t, env.KycApproved)
	require.Equal(t, accID.String(), env.KycApproved.AccountID)
	require.Equal(t, subID.String(), env.KycApproved.SubmissionID)
}

func ptr[T any](v T) *T { return &v }
