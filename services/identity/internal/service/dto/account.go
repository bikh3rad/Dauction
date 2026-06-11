package dto

import "application/internal/entity"

// AccountResp is the read model returned by GET /apis/me and the admin user
// endpoints. All fields are language-neutral (enum codes + booleans); the client
// localizes. `eligible` = MEMBER/VIP AND KYC APPROVED (participation gate).
type AccountResp struct {
	ID             string   `json:"id"`
	Handle         string   `json:"handle"`
	MobileE164     string   `json:"mobileE164"`
	MobileVerified bool     `json:"mobileVerified"`
	Tier           string   `json:"tier"`      // GUEST | MEMBER | VIP
	KycStatus      string   `json:"kycStatus"` // PENDING | APPROVED | REJECTED
	Status         string   `json:"status"`    // REGISTERED | ACTIVE | SUSPENDED | BANNED
	Roles          []string `json:"roles"`     // elevated roles (USER implicit)
	Eligible       bool     `json:"eligible"`
	CreatedAt      string   `json:"createdAt"` // ISO-8601 UTC
	UpdatedAt      string   `json:"updatedAt"` // ISO-8601 UTC
}

// AccessResp is the tier+KYC+roles guard read model the gateway consumes on every
// request to authorize tier/KYC and inspector/admin route groups.
type AccessResp struct {
	ID        string   `json:"id"`
	Tier      string   `json:"tier"`
	KycStatus string   `json:"kycStatus"`
	Eligible  bool     `json:"eligible"`
	Roles     []string `json:"roles"`
}

// ListUsersResp is the admin user listing.
type ListUsersResp struct {
	Users []AccountResp `json:"users"`
	Total int           `json:"total"`
}

// UpdateUserReq is the admin profile-edit body; omitted fields are unchanged.
type UpdateUserReq struct {
	Handle *string `json:"handle"`
	Status *string `json:"status" validate:"omitempty,oneof=REGISTERED ACTIVE SUSPENDED BANNED"`
}

// AssignRoleReq is the role grant/revoke body.
type AssignRoleReq struct {
	Role string `json:"role" validate:"required,oneof=INSPECTOR ADMIN"`
}

func rolesToStrings(roles []entity.Role) []string {
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		out = append(out, string(r))
	}

	return out
}

// ToAccountResp maps an entity.Account to its API response.
func ToAccountResp(a entity.Account) AccountResp {
	return AccountResp{
		ID:             a.ID.String(),
		Handle:         a.Handle,
		MobileE164:     a.MobileE164,
		MobileVerified: a.MobileVerified,
		Tier:           string(a.Tier),
		KycStatus:      string(a.KycStatus),
		Status:         string(a.Status),
		Roles:          rolesToStrings(a.Roles),
		Eligible:       a.Eligible(),
		CreatedAt:      a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ToAccessResp maps an entity.Account to the gateway guard read model.
func ToAccessResp(a entity.Account) AccessResp {
	return AccessResp{
		ID:        a.ID.String(),
		Tier:      string(a.Tier),
		KycStatus: string(a.KycStatus),
		Eligible:  a.Eligible(),
		Roles:     rolesToStrings(a.Roles),
	}
}
