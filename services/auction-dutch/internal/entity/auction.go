package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuctionState is the Dutch auction lifecycle (root CLAUDE.md §3):
// DRAFT -> APPRAISING -> SCHEDULED -> OPEN -> HAMMER -> SETTLING -> COMPLETED,
// with CANCELLED / ABORTED as terminal failure states. MONOSPACE_UPPERCASE
// protocol vocabulary; the value string IS the wire code.
type AuctionState string

const (
	AuctionDraft      AuctionState = "DRAFT"      // freshly created (unused here; reserved)
	AuctionAppraising AuctionState = "APPRAISING" // awaiting appraisal (reserved)
	AuctionScheduled  AuctionState = "SCHEDULED"  // created from lot.scheduled; awaiting open
	AuctionOpen       AuctionState = "OPEN"       // live: price descends; buys accepted
	AuctionHammer     AuctionState = "HAMMER"     // first valid buy hit; winner recorded
	AuctionSettling   AuctionState = "SETTLING"   // funds settling (escrow tail)
	AuctionCompleted  AuctionState = "COMPLETED"  // terminal: settled
	AuctionCancelled  AuctionState = "CANCELLED"  // terminal: pulled before open
	AuctionAborted    AuctionState = "ABORTED"    // terminal: threshold unmet / no buyer
)

// Valid reports whether s is a known auction state.
func (s AuctionState) Valid() bool {
	switch s {
	case AuctionDraft, AuctionAppraising, AuctionScheduled, AuctionOpen,
		AuctionHammer, AuctionSettling, AuctionCompleted, AuctionCancelled, AuctionAborted:
		return true
	default:
		return false
	}
}

// CanOpen reports whether a lot in this state may transition to OPEN. Only a
// SCHEDULED auction may open (root CLAUDE.md §3).
func (s AuctionState) CanOpen() bool { return s == AuctionScheduled }

// CanComplete reports whether this state may transition to COMPLETED. Only a
// SETTLING auction completes.
func (s AuctionState) CanComplete() bool { return s == AuctionSettling }

// CanAbort reports whether this state may transition to ABORTED. An auction may
// be aborted from any non-terminal pre-settlement state.
func (s AuctionState) CanAbort() bool {
	switch s {
	case AuctionDraft, AuctionAppraising, AuctionScheduled, AuctionOpen:
		return true
	default:
		return false
	}
}

// Terminal reports whether the auction has reached an end state.
func (s AuctionState) Terminal() bool {
	switch s {
	case AuctionCompleted, AuctionCancelled, AuctionAborted:
		return true
	default:
		return false
	}
}

// AuctionMode is the engine a lot runs on (root CLAUDE.md §1). This service only
// materializes DUTCH lots; VICKREY/UNIQBID are owned by auction-passive. The
// value string IS the wire code.
type AuctionMode string

const (
	ModeDutch   AuctionMode = "DUTCH"
	ModeVickrey AuctionMode = "VICKREY"
	ModeUniqBid AuctionMode = "UNIQBID"
)

// IsDutch reports whether the mode is the live descending-price engine.
func (m AuctionMode) IsDutch() bool { return m == ModeDutch }

// Tier mirrors identity's access tier on the participation read model. Only
// MEMBER / VIP may participate in (and win) an auction (root CLAUDE.md §1).
type Tier string

const (
	TierGuest  Tier = "GUEST"
	TierMember Tier = "MEMBER"
	TierVIP    Tier = "VIP"
)

// Eligible reports whether a tier may participate (MEMBER or VIP).
func (t Tier) Eligible() bool { return t == TierMember || t == TierVIP }

// Auction is the live descending-price (Dutch) auction aggregate. All money
// fields are int64 USDC cents (root CLAUDE.md §0). The price function is a pure
// function of (CeilingCents, FloorCents, DropStepCents, DropIntervalSeconds,
// OpenAt) evaluated at a server clock; see biz.CurrentPrice.
type Auction struct {
	ID                  uuid.UUID    `json:"id"`
	LotID               uuid.UUID    `json:"lotId"` // from catalog lot.scheduled
	State               AuctionState `json:"state"`
	CeilingCents        int64        `json:"ceilingCents"`        // starting (highest) price
	FloorCents          int64        `json:"floorCents"`          // reserve / lowest price
	DropStepCents       int64        `json:"dropStepCents"`       // amount the price drops each interval
	DropIntervalSeconds int64        `json:"dropIntervalSeconds"` // seconds between drops
	OpenAt              *time.Time   `json:"openAt"`              // clock origin for price(now); nil until OPEN
	HammerAt            *time.Time   `json:"hammerAt"`            // when the lot was hammered
	WinnerAccountID     *uuid.UUID   `json:"winnerAccountId"`     // buyer who hit the hammer
	HammerPriceCents    *int64       `json:"hammerPriceCents"`    // server price at hammer
	CreatedAt           time.Time    `json:"createdAt"`
}

// ReservationState is the per-reservation lifecycle: a request is made
// (REQUESTED), escrow confirms the lock (LOCKED), or it is later released.
type ReservationState string

const (
	ReservationRequested ReservationState = "REQUESTED"
	ReservationLocked    ReservationState = "LOCKED"
	ReservationReleased  ReservationState = "RELEASED"
)

// Valid reports whether s is a known reservation state.
func (s ReservationState) Valid() bool {
	switch s {
	case ReservationRequested, ReservationLocked, ReservationReleased:
		return true
	default:
		return false
	}
}

// ReservationKind distinguishes the 10% reservation deposit from the 100% full
// lock taken before open (root CLAUDE.md §4 escrow path).
type ReservationKind string

const (
	KindDeposit10 ReservationKind = "DEPOSIT_10" // 10% reservation deposit
	KindFullLock  ReservationKind = "FULL_LOCK"  // 100% full lock before open
)

// Valid reports whether k is a known reservation kind.
func (k ReservationKind) Valid() bool {
	switch k {
	case KindDeposit10, KindFullLock:
		return true
	default:
		return false
	}
}

// Reservation is one escrow hold a participant takes on an auction (a 10%
// deposit or a 100% full lock). The escrow service is the funds authority; this
// row tracks the local request/lock state mirror keyed by EscrowRef.
type Reservation struct {
	ID          uuid.UUID        `json:"id"`
	AuctionID   uuid.UUID        `json:"auctionId"`
	AccountID   uuid.UUID        `json:"accountId"`
	Kind        ReservationKind  `json:"kind"`
	AmountCents int64            `json:"amountCents"`
	State       ReservationState `json:"state"`
	EscrowRef   string           `json:"escrowRef"` // idempotency key shared with escrow
	CreatedAt   time.Time        `json:"createdAt"`
}

// Participant is an account's standing in a single auction: its cached
// eligibility (KYC + tier) and the lock states gating a buy (root CLAUDE.md §3).
type Participant struct {
	AuctionID     uuid.UUID        `json:"auctionId"`
	AccountID     uuid.UUID        `json:"accountId"`
	KycApproved   bool             `json:"kycApproved"`
	Tier          Tier             `json:"tier"`
	ReservationSt ReservationState `json:"reservationState"` // DEPOSIT_10 lock state
	FullLockState ReservationState `json:"fullLockState"`    // FULL_LOCK lock state
	JoinedAt      time.Time        `json:"joinedAt"`
}

// Eligible reports whether the participant satisfies every prerequisite to buy:
// KYC approved, an eligible tier, and BOTH the 10% deposit and the 100% full
// lock LOCKED (root CLAUDE.md §3 entry-to-OPEN / buy gate).
func (p Participant) Eligible() bool {
	return p.KycApproved &&
		p.Tier.Eligible() &&
		p.ReservationSt == ReservationLocked &&
		p.FullLockState == ReservationLocked
}
