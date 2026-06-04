package entity

import (
	"time"

	"github.com/google/uuid"
)

// ReasonCode is why a buyer opened a dispute (CLAUDE.md §1, §4).
// MONOSPACE_UPPERCASE; the value string IS the wire code.
type ReasonCode string

const (
	ReasonAuthenticity ReasonCode = "AUTHENTICITY"
	ReasonCondition    ReasonCode = "CONDITION"
	ReasonNotDelivered ReasonCode = "NOT_DELIVERED"
	ReasonOther        ReasonCode = "OTHER"
)

// Valid reports whether r is a known reason code.
func (r ReasonCode) Valid() bool {
	switch r {
	case ReasonAuthenticity, ReasonCondition, ReasonNotDelivered, ReasonOther:
		return true
	default:
		return false
	}
}

// State is the dispute lifecycle (CLAUDE.md §4: dispute court).
// OPEN -> UNDER_REVIEW -> RESOLVED, with WITHDRAWN as a terminal off-ramp.
type State string

const (
	StateOpen        State = "OPEN"
	StateUnderReview State = "UNDER_REVIEW"
	StateResolved    State = "RESOLVED"
	StateWithdrawn   State = "WITHDRAWN"
)

// Valid reports whether s is a known state.
func (s State) Valid() bool {
	switch s {
	case StateOpen, StateUnderReview, StateResolved, StateWithdrawn:
		return true
	default:
		return false
	}
}

// Terminal reports whether s admits no further transitions.
func (s State) Terminal() bool {
	return s == StateResolved || s == StateWithdrawn
}

// NonTerminal reports whether s still allows a second-dispute lock-out
// (only one OPEN/UNDER_REVIEW dispute may exist per trade).
func (s State) NonTerminal() bool {
	return s == StateOpen || s == StateUnderReview
}

// Ruling is the dispute court's verdict (CLAUDE.md §4). NULL until RESOLVED,
// then immutable. Mirrors dauction.common.v1.DisputeRuling.
type Ruling string

const (
	RulingRefundBuyer   Ruling = "REFUND_BUYER"
	RulingReleaseSeller Ruling = "RELEASE_SELLER"
	RulingSplit         Ruling = "SPLIT"
)

// Valid reports whether r is a known ruling.
func (r Ruling) Valid() bool {
	switch r {
	case RulingRefundBuyer, RulingReleaseSeller, RulingSplit:
		return true
	default:
		return false
	}
}

// Action is an immutable audit-trail entry kind (dispute_event.action).
type Action string

const (
	ActionOpened        Action = "OPENED"
	ActionEvidenceAdded Action = "EVIDENCE_ADDED"
	ActionReviewStarted Action = "REVIEW_STARTED"
	ActionRuled         Action = "RULED"
	ActionWithdrawn     Action = "WITHDRAWN"
)

// Dispute is the dispute-court record for a single trade. A buyer (claimant)
// raises an authenticity/condition claim against the seller (respondent)
// post-delivery; the house rules; escrow executes the ruling.
type Dispute struct {
	ID                  uuid.UUID  `json:"id"`
	TradeID             string     `json:"tradeId"` // escrow trade / auction id (free-form external ref)
	ClaimantAccountID   uuid.UUID  `json:"claimantAccountId"`
	RespondentAccountID uuid.UUID  `json:"respondentAccountId"`
	ReasonCode          ReasonCode `json:"reasonCode"`
	State               State      `json:"state"`
	Ruling              *Ruling    `json:"ruling,omitempty"` // nil until RESOLVED
	EvidenceRef         string     `json:"evidenceRef,omitempty"`
	RuledBy             *uuid.UUID `json:"ruledBy,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	ResolvedAt          *time.Time `json:"resolvedAt,omitempty"`
}

// IsParty reports whether account is the claimant or respondent of this dispute.
func (d Dispute) IsParty(account uuid.UUID) bool {
	return d.ClaimantAccountID == account || d.RespondentAccountID == account
}

// DisputeEvent is an immutable audit-trail row. Append-only: never updated or
// deleted (CLAUDE.md §4 "keep an immutable audit trail").
type DisputeEvent struct {
	ID             uuid.UUID `json:"id"`
	DisputeID      uuid.UUID `json:"disputeId"`
	ActorAccountID uuid.UUID `json:"actorAccountId"`
	Action         Action    `json:"action"`
	DetailRef      string    `json:"detailRef,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}
