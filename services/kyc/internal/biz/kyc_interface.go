package biz

import (
	"application/internal/entity"
	"context"
	"time"

	"github.com/google/uuid"
)

// StartParams are the inputs to begin a KYC flow.
type StartParams struct {
	AccountID uuid.UUID
	DocType   entity.DocType
	DocRef    string
	Phone     string
}

// StartResult is returned to the caller after a challenge is issued. DevCode is
// only populated outside production so a developer can complete OTP locally; it
// is never logged or returned in production.
type StartResult struct {
	SubmissionID uuid.UUID
	ChallengeID  uuid.UUID
	ExpiresAt    time.Time
	DevCode      string
}

// RejectParams carries an admin rejection decision. Reason is a machine code.
type RejectParams struct {
	SubmissionID uuid.UUID
	DecidedBy    uuid.UUID
	Reason       string
}

// UsecaseKyc is the KYC use case consumed by the HTTP handlers.
type UsecaseKyc interface {
	// Start begins a new submission and issues an OTP challenge.
	Start(ctx context.Context, p StartParams) (StartResult, error)
	// Verify checks an OTP code for an account's open submission and, on success,
	// moves it to SUBMITTED (entering the admin queue).
	Verify(ctx context.Context, accountID uuid.UUID, code string) (entity.Submission, error)
	// Status returns the account's most recent submission.
	Status(ctx context.Context, accountID uuid.UUID) (entity.Submission, error)
	// PendingQueue lists submissions awaiting an admin decision (SUBMITTED).
	PendingQueue(ctx context.Context) ([]entity.Submission, error)
	// Approve marks a submitted submission APPROVED and emits kyc.approved.
	Approve(ctx context.Context, submissionID, decidedBy uuid.UUID) (entity.Submission, error)
	// Reject marks a submitted submission REJECTED and emits kyc.rejected.
	Reject(ctx context.Context, p RejectParams) (entity.Submission, error)
}

// RepositoryKyc is the persistence seam (implemented by internal/repo, mocked in
// tests). The decision/outbox methods are atomic: the submission write and the
// outbox row commit in a single transaction.
type RepositoryKyc interface {
	// CreateSubmission inserts a new submission (STARTED) and its OTP challenge in
	// one transaction.
	CreateSubmission(ctx context.Context, s entity.Submission, c entity.OTPChallenge) error
	// GetLatestSubmission returns the account's most recent submission.
	GetLatestSubmission(ctx context.Context, accountID uuid.UUID) (entity.Submission, error)
	// GetSubmission returns a submission by id.
	GetSubmission(ctx context.Context, id uuid.UUID) (entity.Submission, error)
	// GetOpenChallenge returns the unverified challenge for a submission.
	GetOpenChallenge(ctx context.Context, submissionID uuid.UUID) (entity.OTPChallenge, error)
	// IncrementChallengeAttempts bumps the attempt counter for a challenge.
	IncrementChallengeAttempts(ctx context.Context, challengeID uuid.UUID) error
	// MarkChallengeVerified flags a challenge verified and advances the submission
	// to OTP_VERIFIED then SUBMITTED in one transaction.
	MarkVerifiedAndSubmit(ctx context.Context, submissionID, challengeID uuid.UUID, submittedAt time.Time) error
	// DecideSubmission writes an APPROVED/REJECTED decision plus an outbox event in
	// one transaction.
	DecideSubmission(ctx context.Context, s entity.Submission, outbox entity.OutboxEvent) error
	// ListByState returns submissions in a given state, newest first.
	ListByState(ctx context.Context, state entity.SubmissionState) ([]entity.Submission, error)

	// Outbox relay support.
	FetchUnpublished(ctx context.Context, limit int) ([]entity.OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID, publishedAt time.Time) error
}

// EventPublisher relays outbox rows onto the message bus (NATS/JetStream).
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}
