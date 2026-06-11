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
	SubjectObjectListed  = "object.listed"
	SubjectCreditChanged = "credit.changed"
	// consumed
	SubjectAuctionCompleted = "auction.completed"
)

const producerName = "vault"

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (vault owns only its folder), so we
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

// money mirrors dauction.common.v1.Money (int64 USDC cents).
type money struct {
	Cents int64 `json:"cents"`
}

// objectListed mirrors dauction.events.v1.ObjectListed. `mode` is the
// MONOSPACE_UPPERCASE AuctionMode name; `duration` is the TimedDuration enum
// name (empty for DUTCH); `floor` is Money.
type objectListed struct {
	ObjectID string `json:"object_id"`
	OwnerID  string `json:"owner_id"`
	Mode     string `json:"mode"`
	Duration string `json:"duration,omitempty"`
	Floor    money  `json:"floor"`
	// Listing content carried downstream so catalog seeds the lot without DB->DB.
	Category     string             `json:"category,omitempty"`
	PrimaryLang  string             `json:"primary_lang,omitempty"`
	Translations []eventTranslation `json:"translations,omitempty"`
	ImageRefs    []string           `json:"image_refs,omitempty"`
}

// eventTranslation is one language's owner-authored content on the event payload.
type eventTranslation struct {
	Lang        string `json:"lang"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// creditChanged mirrors dauction.events.v1.CreditChanged. Delta/Balance are
// signed Money (USDC cents of Vault Credit); reason is a machine code.
type creditChanged struct {
	AccountID string `json:"account_id"`
	Delta     money  `json:"delta"`
	Balance   money  `json:"balance"`
	Reason    string `json:"reason"`
}

// timedDurationName maps the owner-selected duration (in days) to the
// dauction.common.v1.TimedDuration enum value NAME used on the wire. DUTCH (no
// duration) yields the empty string.
func timedDurationName(durationDays int) string {
	switch durationDays {
	case 2: //nolint:mnd
		return "DAYS_2"
	case 5: //nolint:mnd
		return "DAYS_5"
	case 7: //nolint:mnd
		return "DAYS_7"
	default:
		return ""
	}
}

// newEnvelope marshals an EventEnvelope around an already-marshalled payload.
func newEnvelope(subject, idempotencyKey string, payload []byte) ([]byte, error) {
	return json.Marshal(eventEnvelope{
		EventID:        uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		Producer:       producerName,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Type:           subject,
		Version:        1,
		Payload:        payload,
	})
}

// newObjectListedOutbox builds the outbox row + EventEnvelope for an
// object.listed emission. idempotencyKey is producer-stable for the same logical
// listing so consumers dedup. floorCents is the lot floor (the appraised value).
func newObjectListedOutbox(
	objectID, ownerID uuid.UUID,
	mode entity.AuctionMode,
	durationDays int,
	floorCents int64,
	details entity.ListingDetails,
	idempotencyKey string,
) (entity.OutboxEvent, error) {
	translations := make([]eventTranslation, 0, len(details.Translations))
	for _, t := range details.Translations {
		translations = append(translations, eventTranslation{Lang: t.Lang, Title: t.Title, Description: t.Description})
	}

	payload, err := json.Marshal(objectListed{
		ObjectID:     objectID.String(),
		OwnerID:      ownerID.String(),
		Mode:         string(mode),
		Duration:     timedDurationName(durationDays),
		Floor:        money{Cents: floorCents},
		Category:     details.CategoryCode,
		PrimaryLang:  details.PrimaryLang,
		Translations: translations,
		ImageRefs:    details.ImageRefs,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := newEnvelope(SubjectObjectListed, idempotencyKey, payload)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectObjectListed,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}

// newCreditChangedOutbox builds the outbox row + EventEnvelope for a
// credit.changed emission. deltaCents is the signed change just appended to the
// ledger; balanceCents is the resulting balance.
func newCreditChangedOutbox(
	accountID uuid.UUID,
	deltaCents, balanceCents int64,
	reason entity.CreditReason,
	idempotencyKey string,
) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(creditChanged{
		AccountID: accountID.String(),
		Delta:     money{Cents: deltaCents},
		Balance:   money{Cents: balanceCents},
		Reason:    string(reason),
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := newEnvelope(SubjectCreditChanged, idempotencyKey, payload)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectCreditChanged,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}
