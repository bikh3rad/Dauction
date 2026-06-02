package dto

import "application/internal/entity"

// VaultObjectResp is the language-neutral read model for a vault object. Money is
// an integer amount in USDC cents; state is the MONOSPACE_UPPERCASE wire code.
type VaultObjectResp struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	AppraisedValueCents int64  `json:"appraisedValueCents"`
	State               string `json:"state"` // IN_VAULT | APPRAISING | IN_AUCTION | SOLD | BOUGHT_BACK
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

// VaultViewResp is the GET /apis/vault payload: the caller's objects plus their
// derived Vault-Credit balance (USDC cents).
type VaultViewResp struct {
	Objects            []VaultObjectResp `json:"objects"`
	CreditBalanceCents int64             `json:"creditBalanceCents"`
}

// CreateObjectReq is the POST /apis/vault/objects request body.
type CreateObjectReq struct {
	Title               string `json:"title"               validate:"required,max=200"`
	Description         string `json:"description"         validate:"max=2000"`
	AppraisedValueCents int64  `json:"appraisedValueCents" validate:"required,gt=0"`
}

// ListObjectReq is the POST /apis/vault/objects/{id}/list request body.
// DurationDays is required for VICKREY/UNIQBID and forbidden for DUTCH (enforced
// in biz, surfaced as RESOURCE_INVALID).
type ListObjectReq struct {
	Atype        string `json:"atype"                  validate:"required,oneof=DUTCH VICKREY UNIQBID"`
	DurationDays int    `json:"durationDays,omitempty" validate:"omitempty,oneof=2 5 7"`
}

// BuybackReq is the POST /apis/vault/objects/{id}/buyback request body.
type BuybackReq struct {
	Mode string `json:"mode" validate:"required,oneof=CASH CREDIT"`
}

// BuybackResp reports the buyback outcome. PayoutCents is 50% (CASH) or 85%
// (CREDIT) of the appraised value; BalanceCents is the resulting Vault-Credit
// balance (meaningful for CREDIT).
type BuybackResp struct {
	Object       VaultObjectResp `json:"object"`
	Mode         string          `json:"mode"`
	PayoutCents  int64           `json:"payoutCents"`
	BalanceCents int64           `json:"balanceCents"`
}

// isoFormat is the canonical ISO-8601 UTC layout used across the API.
const isoFormat = "2006-01-02T15:04:05Z07:00"

// ToVaultObjectResp maps an entity.VaultObject to its API response.
func ToVaultObjectResp(o entity.VaultObject) VaultObjectResp {
	return VaultObjectResp{
		ID:                  o.ID.String(),
		Title:               o.Title,
		Description:         o.Description,
		AppraisedValueCents: o.AppraisedValueCents,
		State:               string(o.State),
		CreatedAt:           o.CreatedAt.UTC().Format(isoFormat),
		UpdatedAt:           o.UpdatedAt.UTC().Format(isoFormat),
	}
}

// ToVaultObjectResps maps a slice of objects to responses (never nil).
func ToVaultObjectResps(objs []entity.VaultObject) []VaultObjectResp {
	out := make([]VaultObjectResp, 0, len(objs))
	for _, o := range objs {
		out = append(out, ToVaultObjectResp(o))
	}

	return out
}
