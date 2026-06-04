package biz

import (
	"application/internal/entity"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Subject vocabulary on the bus (CLAUDE.md §2, §4).
//
// `dispute.resolved` matches dauction.events.v1.DisputeResolved exactly; escrow
// consumes it to execute the funds movement for the ruling.
//
// `dispute.opened` is NOT in the frozen proto (events.proto only declares
// DisputeResolved). The escrow handoff requires a signal to suspend release
// (HELD -> DISPUTED), so dispute emits a service-local `dispute.opened` envelope
// carrying {trade_id, dispute_id, claimant}. Deviation noted; if the proto gains
// a DisputeOpened arm later, this payload shape is forward-compatible.
const (
	SubjectDisputeOpened   = "dispute.opened"
	SubjectDisputeResolved = "dispute.resolved"
)

const producerName = "dispute"

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (dispute owns only its folder), so we
// marshal the contract shape directly. `payload` carries the single matching arm.
type eventEnvelope struct {
	EventID        string          `json:"event_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Producer       string          `json:"producer"`
	OccurredAt     string          `json:"occurred_at"`
	Type           string          `json:"type"`
	Version        uint32          `json:"version"`
	Payload        json.RawMessage `json:"payload"`
}

// disputeOpened is the service-local dispute.opened payload (no proto arm yet).
type disputeOpened struct {
	DisputeID string `json:"dispute_id"`
	TradeID   string `json:"trade_id"`
	Claimant  string `json:"claimant"`
}

// disputeResolved mirrors dauction.events.v1.DisputeResolved. Ruling is the
// MONOSPACE_UPPERCASE enum name (proto3 JSON encodes enums as names).
type disputeResolved struct {
	DisputeID string `json:"dispute_id"`
	TradeID   string `json:"trade_id"`
	Ruling    string `json:"ruling"`
}

// newOpenedOutbox builds the outbox row + EventEnvelope for a dispute.opened
// emission. idempotencyKey is producer-stable for this logical write so escrow
// dedups (one suspend-release per dispute).
func newOpenedOutbox(disputeID uuid.UUID, tradeID string, claimant uuid.UUID, idempotencyKey string) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(disputeOpened{
		DisputeID: disputeID.String(),
		TradeID:   tradeID,
		Claimant:  claimant.String(),
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return newEnvelopeOutbox(SubjectDisputeOpened, idempotencyKey, payload)
}

// newResolvedOutbox builds the outbox row + EventEnvelope for a dispute.resolved
// emission (escrow consumes it to execute the ruling's funds movement).
func newResolvedOutbox(disputeID uuid.UUID, tradeID string, ruling entity.Ruling, idempotencyKey string) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(disputeResolved{
		DisputeID: disputeID.String(),
		TradeID:   tradeID,
		Ruling:    string(ruling),
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return newEnvelopeOutbox(SubjectDisputeResolved, idempotencyKey, payload)
}

// newEnvelopeOutbox wraps a marshaled payload in an EventEnvelope and returns the
// outbox row to persist in the same transaction as the state change.
func newEnvelopeOutbox(subject, idempotencyKey string, payload json.RawMessage) (entity.OutboxEvent, error) {
	envelope, err := json.Marshal(eventEnvelope{
		EventID:        uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		Producer:       producerName,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Type:           subject,
		Version:        1,
		Payload:        payload,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        subject,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}
