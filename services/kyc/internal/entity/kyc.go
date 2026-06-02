package entity

import (
	"time"

	"github.com/google/uuid"
)

// SubmissionState is the KYC submission state machine.
//
//	STARTED -> OTP_VERIFIED -> SUBMITTED -> APPROVED | REJECTED
//
// A REJECTED submission is terminal; the account may resubmit, which creates a
// brand-new submission row in STARTED.
type SubmissionState string

const (
	SubmissionStarted     SubmissionState = "STARTED"
	SubmissionOTPVerified SubmissionState = "OTP_VERIFIED"
	SubmissionSubmitted   SubmissionState = "SUBMITTED"
	SubmissionApproved    SubmissionState = "APPROVED"
	SubmissionRejected    SubmissionState = "REJECTED"
)

// Valid reports whether s is a known submission state.
func (s SubmissionState) Valid() bool {
	switch s {
	case SubmissionStarted, SubmissionOTPVerified, SubmissionSubmitted,
		SubmissionApproved, SubmissionRejected:
		return true
	default:
		return false
	}
}

// DocType is the kind of identity document referenced by a submission.
type DocType string

const (
	DocEmiratesID DocType = "EMIRATES_ID"
	DocPassport   DocType = "PASSPORT"
)

// Valid reports whether d is a known document type.
func (d DocType) Valid() bool {
	switch d {
	case DocEmiratesID, DocPassport:
		return true
	default:
		return false
	}
}

// Submission is a single KYC attempt for an account. We only store a document
// reference (an external ID/handle), never the document blob itself.
type Submission struct {
	ID              uuid.UUID       `json:"id"`
	AccountID       uuid.UUID       `json:"accountId"`
	DocType         DocType         `json:"docType"`
	DocRef          string          `json:"docRef"`
	Phone           string          `json:"phone"`
	State           SubmissionState `json:"state"`
	RejectionReason string          `json:"rejectionReason,omitempty"`
	DecidedBy       *uuid.UUID      `json:"decidedBy,omitempty"`
	SubmittedAt     time.Time       `json:"submittedAt"`
	DecidedAt       *time.Time      `json:"decidedAt,omitempty"`
}

// OTPChallenge is the one-time-password challenge tied to a submission's phone.
// Only a hash of the code is persisted; the plaintext code is never stored.
type OTPChallenge struct {
	ID          uuid.UUID `json:"id"`
	SubmissionID uuid.UUID `json:"submissionId"`
	Phone       string    `json:"phone"`
	CodeHash    string    `json:"-"`
	Attempts    int       `json:"attempts"`
	Verified    bool      `json:"verified"`
	ExpiresAt   time.Time `json:"expiresAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

// OutboxEvent is a transactional-outbox row: written in the same DB tx as the
// state change it describes, then relayed to NATS by the background publisher.
type OutboxEvent struct {
	ID             uuid.UUID  `json:"id"`
	IdempotencyKey string     `json:"idempotencyKey"`
	Subject        string     `json:"subject"`
	Payload        []byte     `json:"payload"`
	CreatedAt      time.Time  `json:"createdAt"`
	PublishedAt    *time.Time `json:"publishedAt,omitempty"`
}
