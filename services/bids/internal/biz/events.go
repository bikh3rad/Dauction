package biz

import (
	"application/internal/entity"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Subject vocabulary on the bus (CLAUDE.md §2, §5). These are the NATS subjects /
// EventEnvelope.type values this service produces. bids CONSUMES nothing — it is
// called synchronously by auction-passive before a bid is recorded.
const (
	SubjectBidsPurchased = "bids.purchased"
	SubjectBidsDebited   = "bids.debited"
)

const producerName = "bids"

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (bids owns only its folder), so we
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

// bidsPurchased mirrors dauction.events.v1.BidsPurchased. Credits are whole bid
// credits; usdc_cents is USDC cents — distinct units carried in distinct fields.
type bidsPurchased struct {
	AccountID string `json:"account_id"`
	PackageID string `json:"package_id"`
	Credits   int64  `json:"credits_granted"`
	USDCCents int64  `json:"usdc_charged"`
}

// bidsDebited mirrors dauction.events.v1.BidsDebited. `idempotency_key` matches the
// bid write on the auction-passive side so the bid + debit reconcile via outbox.
type bidsDebited struct {
	AccountID      string `json:"account_id"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
	Balance        int64  `json:"balance"`
}

// newPurchasedOutbox builds the outbox row + EventEnvelope for a bids.purchased
// emission. idempotencyKey is producer-stable for the same logical purchase.
func newPurchasedOutbox(
	accountID uuid.UUID,
	packageID string,
	credits, usdcCents int64,
	idempotencyKey string,
) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(bidsPurchased{
		AccountID: accountID.String(),
		PackageID: packageID,
		Credits:   credits,
		USDCCents: usdcCents,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return wrapEnvelope(SubjectBidsPurchased, idempotencyKey, payload)
}

// NewDebitedOutbox builds the outbox row + EventEnvelope for a bids.debited
// emission. idempotencyKey is the same key the debit row is unique on, so the
// emission is exactly-once per logical debit. It is exported because the resulting
// `balance` is only known inside the repo's debit transaction (after the
// conditional UPDATE), so the repo builds this row with the authoritative balance.
func NewDebitedOutbox(
	accountID uuid.UUID,
	amount, balance int64,
	idempotencyKey string,
) (entity.OutboxEvent, error) {
	payload, err := json.Marshal(bidsDebited{
		AccountID:      accountID.String(),
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		Balance:        balance,
	})
	if err != nil {
		return entity.OutboxEvent{}, err
	}

	return wrapEnvelope(SubjectBidsDebited, idempotencyKey, payload)
}

// wrapEnvelope marshals the EventEnvelope shape around a payload and returns the
// outbox row to be written in the same tx as the state change.
func wrapEnvelope(subject, idempotencyKey string, payload json.RawMessage) (entity.OutboxEvent, error) {
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
