package dto

import (
	"application/internal/biz"
	"application/internal/entity"
)

const iso8601 = "2006-01-02T15:04:05Z07:00"

// FundReq is the request DTO for funding a winner's obligation. amountCents must
// equal the trade obligation exactly.
type FundReq struct {
	AmountCents int64 `json:"amountCents" validate:"required,gt=0"`
}

// ConfirmReq is the request DTO for buyer delivery confirmation. mode selects the
// seller payout: CASH (100%) or VAULT_CREDIT (110% recorded as a credit instruction).
type ConfirmReq struct {
	Mode string `json:"mode" validate:"required,oneof=CASH VAULT_CREDIT"`
}

// RefundReq is the admin refund request DTO. participantId is the account whose
// locked funds are returned.
type RefundReq struct {
	ParticipantID string `json:"participantId" validate:"required,uuid"`
}

// BalanceResp is one participant's derived balance for a trade (USDC cents).
type BalanceResp struct {
	ParticipantID string `json:"participantId"`
	BalanceCents  int64  `json:"balanceCents"`
}

// ConservationResp is the funds-conservation summary for a trade (root §4): the
// gross inflows locked into the pot and the gross amount disbursed out.
type ConservationResp struct {
	InflowsCents   int64 `json:"inflowsCents"`
	DisbursedCents int64 `json:"disbursedCents"`
	Balanced       bool  `json:"balanced"`
}

// TradeResp is the read model returned by GET /apis/escrow/{tradeId}: the trade
// head state + derived per-participant balances + conservation summary. All
// amounts are int64 USDC cents; all enums are MONOSPACE_UPPERCASE wire codes.
type TradeResp struct {
	ID                string           `json:"id"`
	LotID             string           `json:"lotId"`
	BuyerID           string           `json:"buyerId"`
	SellerID          string           `json:"sellerId"`
	Kind              string           `json:"kind"`  // DUTCH | PASSIVE
	State             string           `json:"state"` // UNLOCKED | DEPOSIT_LOCKED | ...
	PriceCents        int64            `json:"priceCents"`
	PremiumCents      int64            `json:"premiumCents"`
	FeeCents          int64            `json:"feeCents"`
	InspectorFeeCents int64            `json:"inspectorFeeCents"`
	ObligationCents   int64            `json:"obligationCents"`
	ReleaseMode       string           `json:"releaseMode,omitempty"`
	FundingDeadline   string           `json:"fundingDeadline,omitempty"` // ISO-8601 UTC
	CreatedAt         string           `json:"createdAt"`
	UpdatedAt         string           `json:"updatedAt"`
	Balances          []BalanceResp    `json:"balances"`
	Conservation      ConservationResp `json:"conservation"`
}

// TradeStateResp is the minimal head returned by the mutating endpoints (fund /
// confirm / refund / forfeit).
type TradeStateResp struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	Kind        string `json:"kind"`
	ReleaseMode string `json:"releaseMode,omitempty"`
}

// ToTradeResp maps a biz.TradeView to the full read model.
func ToTradeResp(v biz.TradeView) TradeResp {
	t := v.Trade

	balances := make([]BalanceResp, 0, len(v.Balances))
	for _, b := range v.Balances {
		balances = append(balances, BalanceResp{
			ParticipantID: b.ParticipantAccountID.String(),
			BalanceCents:  b.BalanceCents,
		})
	}

	resp := TradeResp{
		ID:                t.ID.String(),
		LotID:             t.LotID.String(),
		BuyerID:           t.BuyerAccountID.String(),
		SellerID:          t.SellerAccountID.String(),
		Kind:              string(t.Kind),
		State:             string(t.State),
		PriceCents:        t.PriceCents,
		PremiumCents:      t.PremiumCents,
		FeeCents:          t.FeeCents,
		InspectorFeeCents: t.InspectorFeeCents,
		ObligationCents:   t.ObligationCents(),
		ReleaseMode:       string(t.ReleaseMode),
		CreatedAt:         t.CreatedAt.UTC().Format(iso8601),
		UpdatedAt:         t.UpdatedAt.UTC().Format(iso8601),
		Balances:          balances,
		Conservation: ConservationResp{
			InflowsCents:   v.Conservation.Inflows,
			DisbursedCents: v.Conservation.Disbursed,
			Balanced:       v.Conservation.Balanced(),
		},
	}

	if t.FundingDeadline != nil {
		resp.FundingDeadline = t.FundingDeadline.UTC().Format(iso8601)
	}

	return resp
}

// ToTradeStateResp maps an entity.EscrowTrade to the minimal head response.
func ToTradeStateResp(t entity.EscrowTrade) TradeStateResp {
	return TradeStateResp{
		ID:          t.ID.String(),
		State:       string(t.State),
		Kind:        string(t.Kind),
		ReleaseMode: string(t.ReleaseMode),
	}
}
