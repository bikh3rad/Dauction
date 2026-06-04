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
	SubjectAuctionOpened       = "auction.opened"
	SubjectAuctionHammer       = "auction.hammer"
	SubjectAuctionCompleted    = "auction.completed"
	SubjectEscrowLockRequested = "escrow.lock_requested"
	// consumed
	SubjectLotScheduled = "lot.scheduled"
	SubjectEscrowLocked = "escrow.locked"
)

const producerName = "auction-dutch"

// eventEnvelope mirrors dauction.events.v1.EventEnvelope on the wire. The proto
// stubs are not imported into this module (auction-dutch owns only its folder),
// so we marshal the contract shape directly. `payload` carries the matching arm.
type eventEnvelope struct {
	EventID        string          `json:"event_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Producer       string          `json:"producer"`
	OccurredAt     string          `json:"occurred_at"`
	Type           string          `json:"type"`
	Version        uint32          `json:"version"`
	Payload        json.RawMessage `json:"payload"`
}

// money mirrors dauction.common.v1.Money: int64 USDC cents, never a float.
type money struct {
	Cents int64 `json:"cents"`
}

// ---- consumed shapes ----

// lotScheduled mirrors the catalog lot.scheduled payload (events.v1.LotScheduled
// + the extra fields catalog carries). DUTCH lots seed a Dutch auction; the
// catalog-supplied auction_id becomes this auction's id.
type lotScheduled struct {
	LotID        string `json:"lot_id"`
	AuctionID    string `json:"auction_id"`
	ObjectID     string `json:"object_id"`
	Mode         string `json:"mode"` // DUTCH | VICKREY | UNIQBID
	DurationDays int32  `json:"duration_days"`
	ScheduledAt  string `json:"scheduled_at"`
	ReserveCents int64  `json:"reserve_cents"`
	Week         string `json:"week"`
}

// escrowLocked mirrors dauction.events.v1.EscrowLocked. `state` is an EscrowState
// name (DEPOSIT_LOCKED / FULL_LOCKED). `idempotency_key` on the bound reservation
// is carried back as trade_id/participant context; we match on the local
// reservation escrow_ref, which is supplied as `escrow_ref` (an additive field
// the escrow service echoes from escrow.lock_requested — see deviation note).
type escrowLocked struct {
	TradeID       string `json:"trade_id"`
	ParticipantID string `json:"participant_id"`
	State         string `json:"state"` // DEPOSIT_LOCKED | FULL_LOCKED
	Amount        money  `json:"amount"`
	EscrowRef     string `json:"escrow_ref"` // additive: echoes the lock_requested ref
}

// ---- emitted payload shapes ----

// auctionOpened mirrors dauction.events.v1.AuctionOpened. opened_at is the clock
// origin downstream renderers use for price(now).
type auctionOpened struct {
	AuctionID        string `json:"auction_id"`
	LotID            string `json:"lot_id"`
	Ceiling          money  `json:"ceiling"`
	Floor            money  `json:"floor"`
	DropStep         money  `json:"drop_step"`
	DropIntervalSecs uint32 `json:"drop_interval_secs"`
	OpenedAt         string `json:"opened_at"`
}

// auctionHammer mirrors dauction.events.v1.AuctionHammer. premium is 0 here; the
// buyer's premium is applied downstream by escrow on funding.
type auctionHammer struct {
	AuctionID   string `json:"auction_id"`
	LotID       string `json:"lot_id"`
	WinnerID    string `json:"winner_id"`
	HammerPrice money  `json:"hammer_price"`
	Premium     money  `json:"premium"`
	HammeredAt  string `json:"hammered_at"`
}

// auctionCompleted mirrors dauction.events.v1.AuctionCompleted. final_state is an
// AuctionState name (COMPLETED / CANCELLED / ABORTED).
type auctionCompleted struct {
	AuctionID  string `json:"auction_id"`
	LotID      string `json:"lot_id"`
	FinalState string `json:"final_state"`
}

// escrowLockRequested is auction-dutch's ASK to the escrow service to lock funds
// for a participant. NOTE: this subject is NOT in the frozen events.proto oneof
// (deviation, see service CLAUDE.md). The escrow service is expected to consume
// it, lock the funds, and emit escrow.locked echoing `escrow_ref` so we can flip
// the matching local reservation. `kind` is the reservation kind; `escrow_state`
// is the target EscrowState the lock should reach.
type escrowLockRequested struct {
	AuctionID   string `json:"auction_id"`
	AccountID   string `json:"account_id"`
	Kind        string `json:"kind"`         // DEPOSIT_10 | FULL_LOCK
	EscrowState string `json:"escrow_state"` // DEPOSIT_LOCKED | FULL_LOCKED
	Amount      money  `json:"amount"`
	EscrowRef   string `json:"escrow_ref"` // idempotency key; echoed back on escrow.locked
}

// escrowStateForKind maps a reservation kind to the target EscrowState name the
// escrow service should reach (root CLAUDE.md §4).
func escrowStateForKind(kind entity.ReservationKind) string {
	if kind == entity.KindFullLock {
		return "FULL_LOCKED"
	}

	return "DEPOSIT_LOCKED"
}

// newEscrowLockRequestedOutbox builds the outbox row for an escrow.lock_requested
// emission. The reservation's escrow_ref is the producer-stable idempotency key.
func newEscrowLockRequestedOutbox(res entity.Reservation) (entity.OutboxEvent, error) {
	return newOutbox(SubjectEscrowLockRequested, res.EscrowRef, escrowLockRequested{
		AuctionID:   res.AuctionID.String(),
		AccountID:   res.AccountID.String(),
		Kind:        string(res.Kind),
		EscrowState: escrowStateForKind(res.Kind),
		Amount:      money{Cents: res.AmountCents},
		EscrowRef:   res.EscrowRef,
	})
}

// newAuctionOpenedOutbox builds the outbox row for an auction.opened emission.
func newAuctionOpenedOutbox(a entity.Auction, idempotencyKey string) (entity.OutboxEvent, error) {
	openedAt := ""
	if a.OpenAt != nil {
		openedAt = a.OpenAt.UTC().Format(time.RFC3339Nano)
	}

	return newOutbox(SubjectAuctionOpened, idempotencyKey, auctionOpened{
		AuctionID:        a.ID.String(),
		LotID:            a.LotID.String(),
		Ceiling:          money{Cents: a.CeilingCents},
		Floor:            money{Cents: a.FloorCents},
		DropStep:         money{Cents: a.DropStepCents},
		DropIntervalSecs: uint32(a.DropIntervalSeconds), //nolint:gosec // interval fits uint32
		OpenedAt:         openedAt,
	})
}

// newAuctionHammerOutbox builds the outbox row for an auction.hammer emission.
func newAuctionHammerOutbox(a entity.Auction, winner uuid.UUID, priceCents int64, hammerAt time.Time, idempotencyKey string) (entity.OutboxEvent, error) {
	return newOutbox(SubjectAuctionHammer, idempotencyKey, auctionHammer{
		AuctionID:   a.ID.String(),
		LotID:       a.LotID.String(),
		WinnerID:    winner.String(),
		HammerPrice: money{Cents: priceCents},
		Premium:     money{Cents: 0},
		HammeredAt:  hammerAt.UTC().Format(time.RFC3339Nano),
	})
}

// newAuctionCompletedOutbox builds the outbox row for an auction.completed
// emission carrying the final state.
func newAuctionCompletedOutbox(a entity.Auction, finalState entity.AuctionState, idempotencyKey string) (entity.OutboxEvent, error) {
	return newOutbox(SubjectAuctionCompleted, idempotencyKey, auctionCompleted{
		AuctionID:  a.ID.String(),
		LotID:      a.LotID.String(),
		FinalState: string(finalState),
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
