package entity

import (
	"time"

	"github.com/google/uuid"
)

// Tier is the access tier on an account (CLAUDE.md §1). MONOSPACE_UPPERCASE
// protocol vocabulary; the value string IS the wire code. Tier only ever rises:
// GUEST -> MEMBER -> VIP.
type Tier string

const (
	TierGuest  Tier = "GUEST"  // browses gallery, cannot participate
	TierMember Tier = "MEMBER" // KYC-approved (invites removed; KYC is the trigger)
	TierVIP    Tier = "VIP"    // house-granted
)

// rank gives the monotonic ordering used to enforce "tier only rises".
func (t Tier) rank() int {
	switch t {
	case TierGuest:
		return 1
	case TierMember:
		return 2
	case TierVIP:
		return 3
	default:
		return 0
	}
}

// Valid reports whether t is a known tier.
func (t Tier) Valid() bool { return t.rank() != 0 }

// Below reports whether t is strictly lower than other (a legal elevation target).
func (t Tier) Below(other Tier) bool { return t.rank() < other.rank() }

// KycState is the identity-verification status mirrored onto the account from
// the kyc service's events. MONOSPACE_UPPERCASE.
type KycState string

const (
	KycPending  KycState = "PENDING"
	KycApproved KycState = "APPROVED"
	KycRejected KycState = "REJECTED"
)

// Valid reports whether k is a known kyc state.
func (k KycState) Valid() bool {
	switch k {
	case KycPending, KycApproved, KycRejected:
		return true
	default:
		return false
	}
}

// Role is a functional permission grant, orthogonal to Tier (CLAUDE.md §0 rule 8).
// USER is implicit (never stored); INSPECTOR/ADMIN are explicit account_role rows.
type Role string

const (
	RoleInspector Role = "INSPECTOR"
	RoleAdmin     Role = "ADMIN"
)

// Valid reports whether r is a grantable role.
func (r Role) Valid() bool { return r == RoleInspector || r == RoleAdmin }

// Status is the account lifecycle state, orthogonal to tier/role.
type Status string

const (
	StatusRegistered Status = "REGISTERED"
	StatusActive     Status = "ACTIVE"
	StatusSuspended  Status = "SUSPENDED"
	StatusBanned     Status = "BANNED"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusRegistered, StatusActive, StatusSuspended, StatusBanned:
		return true
	default:
		return false
	}
}

// Account is the identity-owned record for a platform user. It carries the
// access tier, a mirrored KYC status, RBAC roles and the mobile identifier so the
// gateway guard can read participation eligibility and role grants from one place.
type Account struct {
	ID             uuid.UUID `json:"id"`
	Handle         string    `json:"handle"`
	MobileE164     string    `json:"mobileE164"`
	MobileVerified bool      `json:"mobileVerified"`
	Tier           Tier      `json:"tier"`
	KycStatus      KycState  `json:"kycStatus"`
	Status         Status    `json:"status"`
	Roles          []Role    `json:"roles"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// HasRole reports whether the account holds the given functional role.
func (a Account) HasRole(role Role) bool {
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}

	return false
}

// Eligible reports whether the account may participate in auctions:
// MEMBER/VIP tier AND KYC approved (CLAUDE.md §1).
func (a Account) Eligible() bool {
	return (a.Tier == TierMember || a.Tier == TierVIP) && a.KycStatus == KycApproved
}
