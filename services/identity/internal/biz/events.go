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
	SubjectAccountRegistered  = "account.registered"
	SubjectAccountTierChanged = "account.tier_changed"
	SubjectAccountRoleChanged = "account.role_changed"
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

// accountRegistered mirrors dauction.events.v1.AccountRegistered.
type accountRegistered struct {
	AccountID     string `json:"account_id"`
	MobileE164    string `json:"mobile_e164"`
	OAuthProvider string `json:"oauth_provider"`
	RegisteredAt  string `json:"registered_at"`
}

// newRegisteredOutbox builds the outbox row + EventEnvelope for account.registered
// (consumed by vault to auto-provision the user's Vault). idempotencyKey is the
// account id so the event fires exactly once per account.
func newRegisteredOutbox(id uuid.UUID, mobile, provider string) (entity.OutboxEvent, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(accountRegistered{
		AccountID:     id.String(),
		MobileE164:    mobile,
		OAuthProvider: provider,
		RegisteredAt:  now,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	key := "identity:registered:" + id.String()
	envelope, err := json.Marshal(eventEnvelope{
		EventID:        uuid.NewString(),
		IdempotencyKey: key,
		Producer:       producerName,
		OccurredAt:     now,
		Type:           SubjectAccountRegistered,
		Version:        1,
		Payload:        payload,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectAccountRegistered,
		IdempotencyKey: key,
		Payload:        envelope,
	}, nil
}

// accountRoleChanged mirrors dauction.events.v1.AccountRoleChanged.
type accountRoleChanged struct {
	AccountID string `json:"account_id"`
	Role      string `json:"role"`
	Granted   bool   `json:"granted"`
	ChangedBy string `json:"changed_by"`
}

// newRoleChangedOutbox builds the outbox row + EventEnvelope for an
// account.role_changed emission. idempotencyKey is producer-stable for the same
// (account, role, granted) write so consumers dedup.
func newRoleChangedOutbox(
	id uuid.UUID, role entity.Role, granted bool, changedBy uuid.UUID, idempotencyKey string,
) (entity.OutboxEvent, error) {
	changer := ""
	if changedBy != uuid.Nil {
		changer = changedBy.String()
	}

	payload, err := json.Marshal(accountRoleChanged{
		AccountID: id.String(),
		Role:      string(role),
		Granted:   granted,
		ChangedBy: changer,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := json.Marshal(eventEnvelope{
		EventID:        uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		Producer:       producerName,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Type:           SubjectAccountRoleChanged,
		Version:        1,
		Payload:        payload,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectAccountRoleChanged,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}
