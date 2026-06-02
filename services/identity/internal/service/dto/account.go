package dto

import "application/internal/entity"

// AccountResp is the read model returned by GET /apis/me and the internal access
// endpoint. All fields are language-neutral (enum codes + booleans); the client
// localizes. `eligible` = MEMBER/VIP AND KYC APPROVED (participation gate).
type AccountResp struct {
	ID        string `json:"id"`
	Tier      string `json:"tier"`      // GUEST | MEMBER | VIP
	KycStatus string `json:"kycStatus"` // PENDING | APPROVED | REJECTED
	Eligible  bool   `json:"eligible"`
	CreatedAt string `json:"createdAt"` // ISO-8601 UTC
	UpdatedAt string `json:"updatedAt"` // ISO-8601 UTC
}

// AccessResp is the minimal tier+KYC guard read model the gateway consumes on
// every request to authorize tier/KYC-gated routes.
type AccessResp struct {
	ID        string `json:"id"`
	Tier      string `json:"tier"`
	KycStatus string `json:"kycStatus"`
	Eligible  bool   `json:"eligible"`
}

// ToAccountResp maps an entity.Account to its API response.
func ToAccountResp(a entity.Account) AccountResp {
	return AccountResp{
		ID:        a.ID.String(),
		Tier:      string(a.Tier),
		KycStatus: string(a.KycStatus),
		Eligible:  a.Eligible(),
		CreatedAt: a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ToAccessResp maps an entity.Account to the gateway guard read model.
func ToAccessResp(a entity.Account) AccessResp {
	return AccessResp{
		ID:        a.ID.String(),
		Tier:      string(a.Tier),
		KycStatus: string(a.KycStatus),
		Eligible:  a.Eligible(),
	}
}
