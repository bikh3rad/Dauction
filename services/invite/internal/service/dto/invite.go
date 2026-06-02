package dto

import "application/internal/entity"

// RedeemInviteReq is the body for POST /apis/invites/redeem.
type RedeemInviteReq struct {
	Code string `json:"code" validate:"required,min=4,max=64"`
}

// RedeemInviteResp confirms a redemption.
type RedeemInviteResp struct {
	Code       string `json:"code"`
	RedeemedBy string `json:"redeemedBy"`
	IssuedBy   string `json:"issuedBy"`
}

// IssueInviteResp is returned when a member/VIP issues a new code.
type IssueInviteResp struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	IssuerAccountID string `json:"issuerAccountId"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
}

// InviteResp is the response DTO for a single invite.
type InviteResp struct {
	ID              string `json:"id"`
	Code            string `json:"code"`
	IssuerAccountID string `json:"issuerAccountId"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
	RedeemedAt      string `json:"redeemedAt,omitempty"`
	RedeemedBy      string `json:"redeemedBy,omitempty"`
}

// ToInviteResp maps an entity.Invite to its response DTO.
func ToInviteResp(e *entity.Invite) *InviteResp {
	if e == nil {
		return nil
	}

	resp := &InviteResp{
		ID:              e.ID.String(),
		Code:            e.Code,
		IssuerAccountID: e.IssuerAccountID,
		Status:          string(e.Status),
		CreatedAt:       e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if e.RedeemedAt != nil {
		resp.RedeemedAt = e.RedeemedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if e.RedeemedBy != nil {
		resp.RedeemedBy = *e.RedeemedBy
	}

	return resp
}

// InviteListResp is a paginated list of invites for the admin console.
type InviteListResp struct {
	Count   int           `json:"count"`
	Invites []*InviteResp `json:"invites"`
}

// ToInviteListResp maps a slice of invites to the list response.
func ToInviteListResp(es []entity.Invite) *InviteListResp {
	resps := make([]*InviteResp, 0, len(es))
	for i := range es {
		resps = append(resps, ToInviteResp(&es[i]))
	}

	return &InviteListResp{Count: len(resps), Invites: resps}
}

// InviteEdgeResp is one link of the invite chain.
type InviteEdgeResp struct {
	Code             string `json:"code"`
	InviterAccountID string `json:"inviterAccountId"`
	InviteeAccountID string `json:"inviteeAccountId"`
	CreatedAt        string `json:"createdAt"`
}

// InviteChainResp is the chain of invitees brought in by an account.
type InviteChainResp struct {
	AccountID string            `json:"accountId"`
	Count     int               `json:"count"`
	Edges     []*InviteEdgeResp `json:"edges"`
}

// ToInviteChainResp maps edges to the chain response.
func ToInviteChainResp(accountID string, es []entity.InviteEdge) *InviteChainResp {
	edges := make([]*InviteEdgeResp, 0, len(es))
	for i := range es {
		edges = append(edges, &InviteEdgeResp{
			Code:             es[i].Code,
			InviterAccountID: es[i].InviterAccountID,
			InviteeAccountID: es[i].InviteeAccountID,
			CreatedAt:        es[i].CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return &InviteChainResp{AccountID: accountID, Count: len(edges), Edges: edges}
}
