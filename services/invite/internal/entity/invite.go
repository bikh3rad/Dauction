package entity

import (
	"time"

	"github.com/google/uuid"
)

// InviteStatus is the lifecycle state of an invite code.
// State machine: ISSUED -> REDEEMED (terminal) | ISSUED -> REVOKED | ISSUED -> FLAGGED.
type InviteStatus string

const (
	InviteStatusIssued   InviteStatus = "ISSUED"
	InviteStatusRedeemed InviteStatus = "REDEEMED"
	InviteStatusRevoked  InviteStatus = "REVOKED"
	InviteStatusFlagged  InviteStatus = "FLAGGED"
)

// Valid reports whether s is a known invite status.
func (s InviteStatus) Valid() bool {
	switch s {
	case InviteStatusIssued, InviteStatusRedeemed, InviteStatusRevoked, InviteStatusFlagged:
		return true
	default:
		return false
	}
}

// Invite is a single-use invitation code. The issuer is the inviter account; on
// redemption the code becomes terminal and an invite_edge chain row is recorded.
type Invite struct {
	ID              uuid.UUID    `json:"id"`
	Code            string       `json:"code"`
	IssuerAccountID string       `json:"issuerAccountId"`
	Status          InviteStatus `json:"status"`
	CreatedAt       time.Time    `json:"createdAt"`
	RedeemedAt      *time.Time   `json:"redeemedAt,omitempty"`
	RedeemedBy      *string      `json:"redeemedBy,omitempty"`
}

// InviteEdge is one link of the invite chain: inviter (issuer) -> invitee (redeemer).
type InviteEdge struct {
	ID               uuid.UUID `json:"id"`
	Code             string    `json:"code"`
	InviterAccountID string    `json:"inviterAccountId"`
	InviteeAccountID string    `json:"inviteeAccountId"`
	CreatedAt        time.Time `json:"createdAt"`
}
