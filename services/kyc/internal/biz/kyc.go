package biz

import (
	"application/internal/entity"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
)

// OTP / event tuning. Defaults follow the brief; overridable via config in the
// constructor.
const (
	DefaultOTPTTL      = 5 * time.Minute
	DefaultMaxAttempts = 5
	otpDigits          = 6

	subjectKycApproved = "kyc.approved"
	subjectKycRejected = "kyc.rejected"
)

// Config tunes OTP behaviour.
type Config struct {
	OTPTTL      time.Duration
	MaxAttempts int
	// Production suppresses dev-code exposure/logging.
	Production bool
}

type kyc struct {
	logger    *slog.Logger
	repo      RepositoryKyc
	publisher EventPublisher
	cfg       Config
	now       func() time.Time
	newID     func() uuid.UUID
}

var _ UsecaseKyc = (*kyc)(nil)

// NewKyc builds the KYC use case.
func NewKyc(logger *slog.Logger, repo RepositoryKyc, publisher EventPublisher, cfg Config) *kyc {
	if cfg.OTPTTL <= 0 {
		cfg.OTPTTL = DefaultOTPTTL
	}

	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultMaxAttempts
	}

	return &kyc{
		logger:    logger.With("layer", "Kyc"),
		repo:      repo,
		publisher: publisher,
		cfg:       cfg,
		now:       time.Now,
		newID:     uuid.New,
	}
}

// Start begins a new KYC submission. If the account already has an APPROVED
// submission, or one still in flight (STARTED/OTP_VERIFIED/SUBMITTED), Start is
// rejected — only a REJECTED (or no) prior submission may begin anew.
func (uc *kyc) Start(ctx context.Context, p StartParams) (StartResult, error) {
	logger := uc.logger.With("method", "Start", "account_id", p.AccountID)

	if p.AccountID == uuid.Nil || !p.DocType.Valid() || p.DocRef == "" || p.Phone == "" {
		return StartResult{}, errors.Join(ErrResourceInvalid, errors.New("invalid start params"))
	}

	if prev, err := uc.repo.GetLatestSubmission(ctx, p.AccountID); err == nil {
		switch prev.State {
		case entity.SubmissionApproved:
			return StartResult{}, errors.Join(ErrResourceExists, errors.New("already approved"))
		case entity.SubmissionStarted, entity.SubmissionOTPVerified, entity.SubmissionSubmitted:
			return StartResult{}, errors.Join(ErrResourceExists, errors.New("submission already in progress"))
		case entity.SubmissionRejected:
			// allowed: resubmit creates a fresh row
		}
	} else if !errors.Is(err, ErrResourceNotFound) {
		return StartResult{}, err
	}

	code, err := genOTP()
	if err != nil {
		return StartResult{}, err
	}

	now := uc.now()
	submissionID := uc.newID()
	challengeID := uc.newID()

	sub := entity.Submission{
		ID:          submissionID,
		AccountID:   p.AccountID,
		DocType:     p.DocType,
		DocRef:      p.DocRef,
		Phone:       p.Phone,
		State:       entity.SubmissionStarted,
		SubmittedAt: now,
	}

	challenge := entity.OTPChallenge{
		ID:           challengeID,
		SubmissionID: submissionID,
		Phone:        p.Phone,
		CodeHash:     hashOTP(code),
		ExpiresAt:    now.Add(uc.cfg.OTPTTL),
		CreatedAt:    now,
	}

	if err := uc.repo.CreateSubmission(ctx, sub, challenge); err != nil {
		logger.ErrorContext(ctx, "create submission failed", "error", err)

		return StartResult{}, err
	}

	res := StartResult{
		SubmissionID: submissionID,
		ChallengeID:  challengeID,
		ExpiresAt:    challenge.ExpiresAt,
	}

	if !uc.cfg.Production {
		res.DevCode = code
		logger.InfoContext(ctx, "DEV OTP issued", "phone", p.Phone, "code", code)
	}

	return res, nil
}

// Verify checks an OTP code against the account's open challenge and advances the
// submission to SUBMITTED.
func (uc *kyc) Verify(ctx context.Context, accountID uuid.UUID, code string) (entity.Submission, error) {
	logger := uc.logger.With("method", "Verify", "account_id", accountID)

	sub, err := uc.repo.GetLatestSubmission(ctx, accountID)
	if err != nil {
		return entity.Submission{}, err
	}

	// Only STARTED submissions accept OTP verification.
	if sub.State != entity.SubmissionStarted {
		return entity.Submission{}, errors.Join(ErrResourceInvalid,
			errors.New("submission not awaiting OTP verification"))
	}

	challenge, err := uc.repo.GetOpenChallenge(ctx, sub.ID)
	if err != nil {
		return entity.Submission{}, err
	}

	now := uc.now()
	if now.After(challenge.ExpiresAt) {
		return entity.Submission{}, errors.Join(ErrResourceInvalid, errors.New("otp expired"))
	}

	if challenge.Attempts >= uc.cfg.MaxAttempts {
		return entity.Submission{}, errors.Join(ErrResourceInvalid, errors.New("otp attempts exceeded"))
	}

	if subtle.ConstantTimeCompare([]byte(hashOTP(code)), []byte(challenge.CodeHash)) != 1 {
		if err := uc.repo.IncrementChallengeAttempts(ctx, challenge.ID); err != nil {
			logger.WarnContext(ctx, "increment attempts failed", "error", err)
		}

		return entity.Submission{}, errors.Join(ErrResourceInvalid, errors.New("otp mismatch"))
	}

	if err := uc.repo.MarkVerifiedAndSubmit(ctx, sub.ID, challenge.ID, now); err != nil {
		logger.ErrorContext(ctx, "mark verified failed", "error", err)

		return entity.Submission{}, err
	}

	sub.State = entity.SubmissionSubmitted
	sub.SubmittedAt = now

	return sub, nil
}

// Status returns the account's latest submission.
func (uc *kyc) Status(ctx context.Context, accountID uuid.UUID) (entity.Submission, error) {
	return uc.repo.GetLatestSubmission(ctx, accountID)
}

// PendingQueue lists submissions awaiting an admin decision.
func (uc *kyc) PendingQueue(ctx context.Context) ([]entity.Submission, error) {
	return uc.repo.ListByState(ctx, entity.SubmissionSubmitted)
}

// Approve transitions SUBMITTED -> APPROVED and writes a kyc.approved outbox row.
func (uc *kyc) Approve(ctx context.Context, submissionID, decidedBy uuid.UUID) (entity.Submission, error) {
	return uc.decide(ctx, submissionID, decidedBy, entity.SubmissionApproved, "")
}

// Reject transitions SUBMITTED -> REJECTED and writes a kyc.rejected outbox row.
func (uc *kyc) Reject(ctx context.Context, p RejectParams) (entity.Submission, error) {
	if p.Reason == "" {
		return entity.Submission{}, errors.Join(ErrResourceInvalid, errors.New("rejection reason required"))
	}

	return uc.decide(ctx, p.SubmissionID, p.DecidedBy, entity.SubmissionRejected, p.Reason)
}

func (uc *kyc) decide(
	ctx context.Context,
	submissionID, decidedBy uuid.UUID,
	target entity.SubmissionState,
	reason string,
) (entity.Submission, error) {
	logger := uc.logger.With("method", "decide", "submission_id", submissionID, "target", target)

	sub, err := uc.repo.GetSubmission(ctx, submissionID)
	if err != nil {
		return entity.Submission{}, err
	}

	// Only a SUBMITTED submission may be decided. Re-deciding an already
	// APPROVED/REJECTED row is an illegal transition.
	if sub.State != entity.SubmissionSubmitted {
		return entity.Submission{}, errors.Join(ErrResourceInvalid,
			errors.New("submission not in SUBMITTED state"))
	}

	now := uc.now()
	sub.State = target
	sub.DecidedBy = &decidedBy
	sub.DecidedAt = &now
	sub.RejectionReason = reason

	outbox, err := buildOutbox(sub, uc.newID(), now)
	if err != nil {
		return entity.Submission{}, err
	}

	if err := uc.repo.DecideSubmission(ctx, sub, outbox); err != nil {
		logger.ErrorContext(ctx, "decide submission failed", "error", err)

		return entity.Submission{}, err
	}

	return sub, nil
}

// genOTP returns a uniformly random 6-digit numeric code (zero-padded).
func genOTP() (string, error) {
	const maxExclusive = 1_000_000

	n, err := rand.Int(rand.Reader, big.NewInt(maxExclusive))
	if err != nil {
		return "", err
	}

	digits := n.Int64()
	out := make([]byte, otpDigits)

	for i := otpDigits - 1; i >= 0; i-- {
		out[i] = byte('0' + digits%10)
		digits /= 10
	}

	return string(out), nil
}

// hashOTP hashes an OTP code; only the hash is ever persisted.
func hashOTP(code string) string {
	sum := sha256.Sum256([]byte(code))

	return hex.EncodeToString(sum[:])
}
