package biz

import (
	"application/internal/entity"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Subject vocabulary on the bus (root CLAUDE.md §2). These are the NATS subjects
// / EventEnvelope.type values this service produces and consumes.
const (
	// emitted
	SubjectEscrowLocked    = "escrow.locked"
	SubjectEscrowReleased  = "escrow.released"
	SubjectEscrowForfeited = "escrow.forfeited"
	SubjectEscrowRefunded  = "escrow.refunded"
	// consumed
	SubjectEscrowLockRequested = "escrow.lock_requested"
	SubjectAuctionHammer       = "auction.hammer"
	SubjectAuctionWon          = "auction.won"
	SubjectDisputeResolved     = "dispute.resolved"
)

const producerName = "escrow"

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (escrow owns only its folder), so we
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

// escrowLocked mirrors dauction.events.v1.EscrowLocked. `state` is the
// MONOSPACE_UPPERCASE EscrowState name (DEPOSIT_LOCKED / FULL_LOCKED / HELD).
type escrowLocked struct {
	TradeID       string `json:"trade_id"`
	ParticipantID string `json:"participant_id"`
	State         string `json:"state"`
	Amount        money  `json:"amount"`
}

// escrowReleased mirrors dauction.events.v1.EscrowReleased. `as_vault_credit`
// false = 100% cash; true = 110% Vault Credit. `credit_cents` is a JSON-only
// extension (proto is narrower) carrying the precomputed 110% instruction so
// vault credits without re-deriving it (deviation noted in CLAUDE.md).
type escrowReleased struct {
	TradeID       string `json:"trade_id"`
	SellerID      string `json:"seller_id"`
	Amount        money  `json:"amount"`
	AsVaultCredit bool   `json:"as_vault_credit"`
	CreditCents   int64  `json:"credit_cents,omitempty"`
}

// escrowForfeited mirrors dauction.events.v1.EscrowForfeited.
type escrowForfeited struct {
	TradeID       string `json:"trade_id"`
	ParticipantID string `json:"participant_id"`
	Amount        money  `json:"amount"`
}

// escrowRefunded is a JSON-only event (proto has no EscrowRefunded arm; §2 lists
// `escrow.refunded` as emitted). Shape mirrors the forfeited/locked family.
type escrowRefunded struct {
	TradeID       string `json:"trade_id"`
	ParticipantID string `json:"participant_id"`
	Amount        money  `json:"amount"`
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

// newEscrowLockedOutbox builds the outbox row + EventEnvelope for an
// escrow.locked emission (DEPOSIT_LOCKED / FULL_LOCKED / HELD).
func newEscrowLockedOutbox(tradeID, participant uuid.UUID, state entity.EscrowState, amountCents int64, idempotencyKey string) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(escrowLocked{
		TradeID:       tradeID.String(),
		ParticipantID: participant.String(),
		State:         string(state),
		Amount:        money{Cents: amountCents},
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := newEnvelope(SubjectEscrowLocked, idempotencyKey, payload)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectEscrowLocked,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}

// newAuctionHeldLockedOutbox builds an escrow.locked emission for the HELD
// transition (Dutch hammer). Kept distinct only by its idempotency key.
func newAuctionHeldLockedOutbox(tradeID, participant uuid.UUID, amountCents int64, idempotencyKey string) (entity.OutboxEvent, error) {
	return newEscrowLockedOutbox(tradeID, participant, entity.StateHeld, amountCents, idempotencyKey)
}

// newEscrowReleasedOutbox builds the outbox row + EventEnvelope for an
// escrow.released emission. When mode is VAULT_CREDIT the 110% credit instruction
// is computed and carried so vault credits the seller.
func newEscrowReleasedOutbox(tradeID, seller uuid.UUID, amountCents int64, mode entity.ReleaseMode, idempotencyKey string) (entity.OutboxEvent, error) {
	asVaultCredit := mode == entity.ReleaseVaultCredit

	var creditCents int64
	if asVaultCredit {
		creditCents = ReleaseCreditCents(amountCents)
	}

	payload, err := json.Marshal(escrowReleased{
		TradeID:       tradeID.String(),
		SellerID:      seller.String(),
		Amount:        money{Cents: amountCents},
		AsVaultCredit: asVaultCredit,
		CreditCents:   creditCents,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := newEnvelope(SubjectEscrowReleased, idempotencyKey, payload)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectEscrowReleased,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}

// newEscrowForfeitedOutbox builds the outbox row + EventEnvelope for an
// escrow.forfeited emission.
func newEscrowForfeitedOutbox(tradeID, participant uuid.UUID, amountCents int64, idempotencyKey string) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(escrowForfeited{
		TradeID:       tradeID.String(),
		ParticipantID: participant.String(),
		Amount:        money{Cents: amountCents},
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := newEnvelope(SubjectEscrowForfeited, idempotencyKey, payload)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectEscrowForfeited,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}

// newEscrowRefundedOutbox builds the outbox row + EventEnvelope for an
// escrow.refunded emission.
func newEscrowRefundedOutbox(tradeID, participant uuid.UUID, amountCents int64, idempotencyKey string) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(escrowRefunded{
		TradeID:       tradeID.String(),
		ParticipantID: participant.String(),
		Amount:        money{Cents: amountCents},
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	envelope, err := newEnvelope(SubjectEscrowRefunded, idempotencyKey, payload)
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return entity.OutboxEvent{
		ID:             uuid.New(),
		Subject:        SubjectEscrowRefunded,
		IdempotencyKey: idempotencyKey,
		Payload:        envelope,
	}, nil
}
