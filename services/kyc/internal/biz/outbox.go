package biz

import (
	"application/internal/entity"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// eventEnvelope mirrors dauction.events.v1.EventEnvelope (proto3 JSON encoding:
// lowerCamelCase field names, enum reason as its value-name string). We emit the
// matching oneof arm (kycApproved / kycRejected) keyed by the event type.
type eventEnvelope struct {
	EventID        string          `json:"eventId"`
	IdempotencyKey string          `json:"idempotencyKey"`
	Producer       string          `json:"producer"`
	OccurredAt     string          `json:"occurredAt"`
	Type           string          `json:"type"`
	Version        uint32          `json:"version"`
	KycApproved    *kycApprovedMsg `json:"kycApproved,omitempty"`
	KycRejected    *kycRejectedMsg `json:"kycRejected,omitempty"`
}

type kycApprovedMsg struct {
	AccountID    string `json:"accountId"`
	SubmissionID string `json:"submissionId"`
}

type kycRejectedMsg struct {
	AccountID    string `json:"accountId"`
	SubmissionID string `json:"submissionId"`
	Reason       string `json:"reason"` // ErrorCode value-name, e.g. KYC_REJECTED
}

const (
	producerName  = "kyc"
	eventVersion  = 1
	defaultReason = "KYC_REJECTED"
)

// buildOutbox renders the outbox row (with its EventEnvelope payload) for a
// decided submission. The idempotency key is stable per (submission, decision)
// so re-publishes dedup downstream.
func buildOutbox(sub entity.Submission, eventID uuid.UUID, now time.Time) (entity.OutboxEvent, error) {
	var (
		subject string
		env     eventEnvelope
	)

	occurred := now.UTC().Format(time.RFC3339)

	switch sub.State {
	case entity.SubmissionApproved:
		subject = subjectKycApproved
		env = eventEnvelope{
			Type: subject,
			KycApproved: &kycApprovedMsg{
				AccountID:    sub.AccountID.String(),
				SubmissionID: sub.ID.String(),
			},
		}
	case entity.SubmissionRejected:
		reason := sub.RejectionReason
		if reason == "" {
			reason = defaultReason
		}

		subject = subjectKycRejected
		env = eventEnvelope{
			Type: subject,
			KycRejected: &kycRejectedMsg{
				AccountID:    sub.AccountID.String(),
				SubmissionID: sub.ID.String(),
				Reason:       reason,
			},
		}
	default:
		return entity.OutboxEvent{}, ErrResourceInvalid
	}

	idemKey := sub.ID.String() + ":" + string(sub.State)

	env.EventID = eventID.String()
	env.IdempotencyKey = idemKey
	env.Producer = producerName
	env.OccurredAt = occurred
	env.Version = eventVersion

	payload, err := json.Marshal(env)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             eventID,
		IdempotencyKey: idemKey,
		Subject:        subject,
		Payload:        payload,
		CreatedAt:      now,
	}, nil
}
