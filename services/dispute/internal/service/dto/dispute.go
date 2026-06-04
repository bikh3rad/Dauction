package dto

import "application/internal/entity"

// OpenDisputeReq is the body of POST /apis/escrow/{tradeId}/dispute. The buyer
// (claimant) is taken from X-Account-Id; the respondent (seller) is supplied by
// the gateway/escrow read since dispute does not own the trade record.
type OpenDisputeReq struct {
	ReasonCode  string `json:"reasonCode"  validate:"required,oneof=AUTHENTICITY CONDITION NOT_DELIVERED OTHER"`
	EvidenceRef string `json:"evidenceRef" validate:"omitempty,max=512"`
	// Respondent is the seller account UUID (the party the claim is against).
	Respondent string `json:"respondent"  validate:"required,uuid"`
}

// AddEvidenceReq is the body of POST /apis/escrow/{tradeId}/dispute/evidence.
type AddEvidenceReq struct {
	DetailRef string `json:"detailRef" validate:"required,max=512"`
}

// ResolveReq is the body of POST /apis/escrow/{tradeId}/dispute/resolve.
type ResolveReq struct {
	Ruling  string `json:"ruling"  validate:"required,oneof=REFUND_BUYER RELEASE_SELLER SPLIT"`
	RuledBy string `json:"ruledBy" validate:"required,uuid"`
}

// DisputeResp is the language-neutral dispute read model (enum codes + ISO-8601
// UTC timestamps; the client localizes).
type DisputeResp struct {
	ID          string `json:"id"`
	TradeID     string `json:"tradeId"`
	Claimant    string `json:"claimant"`
	Respondent  string `json:"respondent"`
	ReasonCode  string `json:"reasonCode"`
	State       string `json:"state"`
	Ruling      string `json:"ruling,omitempty"`
	EvidenceRef string `json:"evidenceRef,omitempty"`
	RuledBy     string `json:"ruledBy,omitempty"`
	CreatedAt   string `json:"createdAt"`
	ResolvedAt  string `json:"resolvedAt,omitempty"`
}

// DisputeEventResp is one immutable audit-trail entry.
type DisputeEventResp struct {
	ID        string `json:"id"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	DetailRef string `json:"detailRef,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// DisputeDetailResp is a dispute plus its audit trail (the GET response).
type DisputeDetailResp struct {
	Dispute DisputeResp        `json:"dispute"`
	Events  []DisputeEventResp `json:"events"`
}

// DisputeListResp is the admin queue response.
type DisputeListResp struct {
	Count    int           `json:"count"`
	Disputes []DisputeResp `json:"disputes"`
}

const isoFormat = "2006-01-02T15:04:05Z07:00"

// ToDisputeResp maps an entity.Dispute to its API response.
func ToDisputeResp(d entity.Dispute) DisputeResp {
	resp := DisputeResp{
		ID:          d.ID.String(),
		TradeID:     d.TradeID,
		Claimant:    d.ClaimantAccountID.String(),
		Respondent:  d.RespondentAccountID.String(),
		ReasonCode:  string(d.ReasonCode),
		State:       string(d.State),
		EvidenceRef: d.EvidenceRef,
		CreatedAt:   d.CreatedAt.UTC().Format(isoFormat),
	}

	if d.Ruling != nil {
		resp.Ruling = string(*d.Ruling)
	}

	if d.RuledBy != nil {
		resp.RuledBy = d.RuledBy.String()
	}

	if d.ResolvedAt != nil {
		resp.ResolvedAt = d.ResolvedAt.UTC().Format(isoFormat)
	}

	return resp
}

// ToDisputeEventResp maps an entity.DisputeEvent to its API response.
func ToDisputeEventResp(e entity.DisputeEvent) DisputeEventResp {
	return DisputeEventResp{
		ID:        e.ID.String(),
		Actor:     e.ActorAccountID.String(),
		Action:    string(e.Action),
		DetailRef: e.DetailRef,
		CreatedAt: e.CreatedAt.UTC().Format(isoFormat),
	}
}

// ToDisputeEventResps maps a slice of audit rows.
func ToDisputeEventResps(es []entity.DisputeEvent) []DisputeEventResp {
	out := make([]DisputeEventResp, 0, len(es))
	for i := range es {
		out = append(out, ToDisputeEventResp(es[i]))
	}

	return out
}

// ToDisputeListResp maps the admin queue.
func ToDisputeListResp(ds []entity.Dispute) DisputeListResp {
	out := make([]DisputeResp, 0, len(ds))
	for i := range ds {
		out = append(out, ToDisputeResp(ds[i]))
	}

	return DisputeListResp{Count: len(out), Disputes: out}
}
