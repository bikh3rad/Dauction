package biz

import (
	"application/internal/entity"
	"context"
	"time"

	"github.com/google/uuid"
)

// ReserveInput is a participant's request to lock funds on an auction: either the
// 10% reservation deposit (KindDeposit10) or the 100% full lock (KindFullLock).
// Tier/KYC are carried from the gateway (header-injected eligibility, see
// EventConsumer doc / service CLAUDE.md) so the auction can cache the
// participant's standing without reaching into identity's DB.
type ReserveInput struct {
	AccountID   uuid.UUID
	Tier        entity.Tier
	KycApproved bool
}

// AuctionView is the read model for GET /apis/auctions/{id}: the auction plus the
// server-computed current price + next drop instant at the read clock.
type AuctionView struct {
	Auction      entity.Auction
	CurrentPrice int64
	NextDropAt   *time.Time
}

// UsecaseAuction is the Dutch auction engine consumed by handlers and the event
// consumer. It owns reservation/full-lock requests, the server-authoritative buy
// (hammer), and the admin open/complete/abort transitions (root CLAUDE.md §3).
type UsecaseAuction interface {
	// Get returns the auction plus its current_price(now) + next_drop_at (public).
	Get(ctx context.Context, id uuid.UUID) (AuctionView, error)

	// Reserve records a participant's 10% reservation deposit request: creates a
	// REQUESTED reservation row, upserts the participant, and emits
	// escrow.lock_requested {kind: DEPOSIT_10}. Idempotent per (auction, account,
	// kind) via the reservation's escrow_ref.
	Reserve(ctx context.Context, auctionID uuid.UUID, in ReserveInput) (entity.Reservation, error)
	// Lock records a participant's 100% full-lock request before open: creates a
	// REQUESTED reservation row and emits escrow.lock_requested {kind: FULL_LOCK}.
	Lock(ctx context.Context, auctionID uuid.UUID, in ReserveInput) (entity.Reservation, error)

	// Buy is THE hammer action. It re-computes current_price server-side, validates
	// the auction is OPEN and the caller is a fully eligible participant (KYC ∧
	// tier ∈ {MEMBER,VIP} ∧ deposit LOCKED ∧ full_lock LOCKED), then atomically
	// transitions OPEN -> HAMMER recording the winner + server price (first valid
	// buy wins; later buys -> ErrResourceInvalid). Emits auction.hammer.
	Buy(ctx context.Context, auctionID uuid.UUID, accountID uuid.UUID) (entity.Auction, error)

	// Open transitions a SCHEDULED auction to OPEN (admin), requiring at least one
	// fully-eligible participant. Sets open_at (the price clock origin) and emits
	// auction.opened. Illegal -> ErrResourceInvalid.
	Open(ctx context.Context, id uuid.UUID) (entity.Auction, error)
	// Complete transitions a SETTLING auction to COMPLETED (admin) and emits
	// auction.completed. Illegal -> ErrResourceInvalid.
	Complete(ctx context.Context, id uuid.UUID) (entity.Auction, error)
	// Abort transitions a non-terminal pre-settlement auction to ABORTED (admin)
	// and emits auction.completed {final_state: ABORTED}. Illegal -> ErrResourceInvalid.
	Abort(ctx context.Context, id uuid.UUID) (entity.Auction, error)

	// CreateFromLotScheduled creates a SCHEDULED auction from a catalog
	// lot.scheduled event (DUTCH only). Idempotent on idempotencyKey via the inbox;
	// a duplicate (or non-DUTCH mode) is a no-op success.
	CreateFromLotScheduled(ctx context.Context, in LotScheduledInput, idempotencyKey string) error
	// ApplyEscrowLocked flips the matching REQUESTED reservation to LOCKED and
	// updates the participant's lock flags, on an escrow.locked event. Idempotent on
	// idempotencyKey; an unknown escrow_ref is a no-op success.
	ApplyEscrowLocked(ctx context.Context, in EscrowLockedInput, idempotencyKey string) error
}

// LotScheduledInput is the auction-internal projection of a catalog lot.scheduled
// event used to seed a SCHEDULED Dutch auction. Only DUTCH lots are materialized.
type LotScheduledInput struct {
	AuctionID    uuid.UUID
	LotID        uuid.UUID
	Mode         entity.AuctionMode
	ReserveCents int64 // becomes the auction floor
}

// EscrowLockedInput is the projection of an escrow.locked event: escrow has
// confirmed a hold, identified by EscrowRef (the reservation's idempotency key).
type EscrowLockedInput struct {
	EscrowRef     string
	TradeID       string
	ParticipantID uuid.UUID
	State         string // DEPOSIT_LOCKED | FULL_LOCKED (escrow vocabulary)
	AmountCents   int64
}

// RepositoryAuction is the persistence seam (implemented by internal/repo, mocked
// in tests). State mutations that emit an event do so atomically with an outbox
// row via the *Tx methods (root CLAUDE.md §0 outbox).
type RepositoryAuction interface {
	// Get returns the auction or ErrResourceNotFound.
	Get(ctx context.Context, id uuid.UUID) (entity.Auction, error)
	// GetParticipant returns the participant or ErrResourceNotFound.
	GetParticipant(ctx context.Context, auctionID, accountID uuid.UUID) (entity.Participant, error)
	// CountEligibleParticipants returns how many participants of the auction are
	// fully eligible (KYC ∧ tier ∧ both locks LOCKED) — the entry-to-OPEN gate.
	CountEligibleParticipants(ctx context.Context, auctionID uuid.UUID) (int, error)

	// CreateAuctionTx inserts a SCHEDULED auction AND marks the inbox key consumed
	// in one tx. Returns ErrResourceExists when the inbox key was already seen
	// (duplicate event) or the auction already exists, so the use case can no-op.
	CreateAuctionTx(ctx context.Context, a entity.Auction, inboxKey string) error

	// ReserveTx upserts the participant, inserts the REQUESTED reservation, and
	// writes the outbox event, all in one tx. Idempotent on the reservation's
	// escrow_ref (ON CONFLICT DO NOTHING); a duplicate returns the existing row.
	ReserveTx(ctx context.Context, p entity.Participant, res entity.Reservation, outbox entity.OutboxEvent) (entity.Reservation, error)

	// ApplyEscrowLockedTx flips the reservation identified by escrowRef from
	// REQUESTED to LOCKED, updates the participant's matching lock flag, and marks
	// the inbox key consumed, all in one tx. Returns ErrResourceExists on a
	// duplicate inbox key, and ErrResourceNotFound when no reservation matches
	// escrowRef (the use case treats the latter as a no-op).
	ApplyEscrowLockedTx(ctx context.Context, escrowRef string, inboxKey string) error

	// HammerTx atomically transitions OPEN -> HAMMER recording winner + price, and
	// writes the outbox event, conditional on the row still being OPEN. A 0-row
	// update (already hammered / not OPEN) -> ErrResourceInvalid. First valid buy wins.
	HammerTx(ctx context.Context, auctionID, winner uuid.UUID, priceCents int64, hammerAt time.Time, outbox entity.OutboxEvent) (entity.Auction, error)

	// OpenTx atomically transitions SCHEDULED -> OPEN setting open_at, and writes
	// the outbox event, conditional on the row still being SCHEDULED. 0-row -> ErrResourceInvalid.
	OpenTx(ctx context.Context, auctionID uuid.UUID, openAt time.Time, outbox entity.OutboxEvent) (entity.Auction, error)
	// TransitionTx atomically flips the auction to `to`, conditional on it being in
	// `from`, and writes the outbox event. 0-row -> ErrResourceInvalid.
	TransitionTx(ctx context.Context, auctionID uuid.UUID, from, to entity.AuctionState, outbox entity.OutboxEvent) (entity.Auction, error)
}
