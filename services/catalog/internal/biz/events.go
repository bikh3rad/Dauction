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
	// emitted
	SubjectAttestationRecorded = "attestation.recorded"
	SubjectLotCertified        = "lot.certified"
	SubjectLotScheduled        = "lot.scheduled"
	// consumed
	SubjectObjectListed = "object.listed"
)

const producerName = "catalog"

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (catalog owns only its folder), so we
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

// objectListed mirrors dauction.events.v1.ObjectListed. Enum/duration values are
// the MONOSPACE_UPPERCASE names (proto3 JSON encodes enums as names). The vault
// emits this when an owner lists a vault object for auction.
type objectListed struct {
	ObjectID string `json:"object_id"`
	OwnerID  string `json:"owner_id"`
	Mode     string `json:"mode"`     // DUTCH | VICKREY | UNIQBID
	Duration string `json:"duration"` // DAYS_2 | DAYS_5 | DAYS_7; empty for DUTCH
	Floor    money  `json:"floor"`
	// Appraised carries the owner's appraised value (USDC cents) so catalog can
	// seed the lot's appraised_value; absent on legacy events (defaults to floor).
	Appraised *money `json:"appraised,omitempty"`
}

// attestationRecorded mirrors dauction.events.v1.AttestationRecorded.
type attestationRecorded struct {
	AttestationID string `json:"attestation_id"`
	LotID         string `json:"lot_id"`
	Inspector     string `json:"inspector"`
	Pass          bool   `json:"pass"`
}

// lotCertified mirrors dauction.events.v1.LotCertified.
type lotCertified struct {
	LotID         string `json:"lot_id"`
	AttestationID string `json:"attestation_id"`
}

// lotScheduled mirrors dauction.events.v1.LotScheduled. We carry the atype +
// timing + reserve so the auction services can pick up the lot without reaching
// into catalog's DB (CLAUDE.md §2: integrate via events, never DB->DB).
type lotScheduled struct {
	LotID        string `json:"lot_id"`
	ObjectID     string `json:"object_id"`
	Mode         string `json:"mode"`          // DUTCH | VICKREY | UNIQBID (atype)
	DurationDays int32  `json:"duration_days"` // 0 for DUTCH
	ScheduledAt  string `json:"scheduled_at"`  // ISO-8601 UTC
	ReserveCents int64  `json:"reserve_cents"`
	Week         string `json:"week"` // ISO week, e.g. "2026-W23"
}

// money mirrors dauction.common.v1.Money: int64 USDC cents, never a float.
type money struct {
	Cents int64 `json:"cents"`
}

// newAttestationRecordedOutbox builds the outbox row + envelope for an
// attestation.recorded emission. idempotencyKey is producer-stable (the
// attestation id) so consumers dedup.
func newAttestationRecordedOutbox(
	attestationID, lotID, inspectorID uuid.UUID,
	pass bool,
	idempotencyKey string,
) (entity.OutboxEvent, error) {
	return newOutbox(SubjectAttestationRecorded, idempotencyKey, attestationRecorded{
		AttestationID: attestationID.String(),
		LotID:         lotID.String(),
		Inspector:     inspectorID.String(),
		Pass:          pass,
	})
}

// newLotCertifiedOutbox builds the outbox row + envelope for a lot.certified
// emission. idempotencyKey is producer-stable per lot certification.
func newLotCertifiedOutbox(lotID, attestationID uuid.UUID, idempotencyKey string) (entity.OutboxEvent, error) {
	return newOutbox(SubjectLotCertified, idempotencyKey, lotCertified{
		LotID:         lotID.String(),
		AttestationID: attestationID.String(),
	})
}

// newLotScheduledOutbox builds the outbox row + envelope for a lot.scheduled
// emission, carrying the atype + params downstream auctions need.
func newLotScheduledOutbox(l entity.Lot, idempotencyKey string) (entity.OutboxEvent, error) {
	var days int32
	if l.DurationDays != nil {
		days = *l.DurationDays
	}

	scheduledAt := ""
	if l.ScheduledAt != nil {
		scheduledAt = l.ScheduledAt.UTC().Format(time.RFC3339Nano)
	}

	return newOutbox(SubjectLotScheduled, idempotencyKey, lotScheduled{
		LotID:        l.ID.String(),
		ObjectID:     l.ObjectID.String(),
		Mode:         string(l.Mode),
		DurationDays: days,
		ReserveCents: l.ReserveCents,
		Week:         l.ISOWeek,
		ScheduledAt:  scheduledAt,
	})
}

// newOutbox wraps a payload in an EventEnvelope and an outbox row for `subject`.
func newOutbox(subject, idempotencyKey string, payload any) (entity.OutboxEvent, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := json.Marshal(eventEnvelope{
		EventID:        uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		Producer:       producerName,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Type:           subject,
		Version:        1,
		Payload:        body,
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

// durationDaysFromProto maps a proto TimedDuration enum name to whole days.
// Returns 0 for DUTCH / unset.
func durationDaysFromProto(d string) int32 {
	switch d {
	case "DAYS_2":
		return 2
	case "DAYS_5":
		return 5
	case "DAYS_7":
		return 7
	default:
		return 0
	}
}
