package entity

// Tier is the access tier enum (root CLAUDE.md §1). Value string IS the wire code.
type Tier string

const (
	TierGuest  Tier = "GUEST"
	TierMember Tier = "MEMBER"
	TierVIP    Tier = "VIP"
)

// KycStatus mirrors identity's kyc_status enum.
type KycStatus string

const (
	KycPending  KycStatus = "PENDING"
	KycApproved KycStatus = "APPROVED"
	KycRejected KycStatus = "REJECTED"
)

// Access is the gateway guard read model fetched from identity's
// GET /apis/internal/accounts/{id}/access. It carries the minimal tier + KYC
// signal the gateway needs to authorize participation routes. The gateway owns
// no DB — this is a transient, briefly-cached projection of identity's truth.
type Access struct {
	ID        string
	Tier      Tier
	KycStatus KycStatus
	Eligible  bool
	// Roles are the caller's elevated functional roles (e.g. INSPECTOR, ADMIN).
	// USER is implicit and omitted. Drives the inspector/admin route groups.
	Roles []string
}

// IsMember reports whether the account is at least MEMBER (MEMBER or VIP).
func (a Access) IsMember() bool {
	return a.Tier == TierMember || a.Tier == TierVIP
}

// HasRole reports whether the account holds the given functional role.
func (a Access) HasRole(role string) bool {
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}

	return false
}

// KycApproved reports whether the account's KYC is approved.
func (a Access) KycApproved() bool {
	return a.KycStatus == KycApproved
}
