package biz

import (
	"application/internal/entity"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Subject vocabulary on the bus (CLAUDE.md §2). These are the NATS subjects /
// EventEnvelope.type values this service produces and consumes.
const (
	SubjectAccountTierChanged = "account.tier_changed"
	SubjectInviteRedeemed     = "invite.redeemed"
	SubjectKycApproved        = "kyc.approved"
)

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (identity owns only its folder), so we
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

// accountTierChanged mirrors dauction.events.v1.AccountTierChanged. Tier values
// are the MONOSPACE_UPPERCASE enum names (proto3 JSON encodes enums as names).
type accountTierChanged struct {
	AccountID string `json:"account_id"`
	From      string `json:"from"`
	To        string `json:"to"`
}

const producerName = "identity"

// newTierChangedOutbox builds the outbox row + EventEnvelope for an
// account.tier_changed emission. idempotencyKey is producer-stable for the same
// logical write so consumers dedup.
func newTierChangedOutbox(id uuid.UUID, from, to entity.Tier, idempotencyKey string) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(accountTierChanged{
		AccountID: id.String(),
		From:      string(from),
		To:        string(to),
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := json.Marshal(eventEnvelope{
		EventID:        uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		Producer:       producerName,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Type:           SubjectAccountTierChanged,
		Version:        1,
		Payload:        payload,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectAccountTierChanged,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}
