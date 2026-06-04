package dto

import (
	"application/internal/biz"
	"application/internal/entity"
	"time"
)

// isoTime formats a time as ISO-8601 UTC, the API's only time encoding.
func isoTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z07:00")
}

// AuctionResp is the language-neutral auction read model (enum codes, integer
// cents, ISO-8601 UTC). The client localizes; the API never pre-formats money or
// dates. currentPriceCents + nextDropAt are SERVER-computed at read time.
type AuctionResp struct {
	ID                  string `json:"id"`
	LotID               string `json:"lotId"`
	State               string `json:"state"` // DRAFT | ... | OPEN | HAMMER | ...
	CeilingCents        int64  `json:"ceilingCents"`
	FloorCents          int64  `json:"floorCents"`
	DropStepCents       int64  `json:"dropStepCents"`
	DropIntervalSeconds int64  `json:"dropIntervalSeconds"`
	CurrentPriceCents   int64  `json:"currentPriceCents"` // server-authoritative price(now)
	OpenAt              string `json:"openAt,omitempty"`
	NextDropAt          string `json:"nextDropAt,omitempty"`
	HammerAt            string `json:"hammerAt,omitempty"`
	WinnerAccountID     string `json:"winnerAccountId,omitempty"`
	HammerPriceCents    *int64 `json:"hammerPriceCents,omitempty"`
	CreatedAt           string `json:"createdAt"`
}

// ReservationResp is the read model for a reservation/full-lock request.
type ReservationResp struct {
	ID          string `json:"id"`
	AuctionID   string `json:"auctionId"`
	AccountID   string `json:"accountId"`
	Kind        string `json:"kind"`  // DEPOSIT_10 | FULL_LOCK
	State       string `json:"state"` // REQUESTED | LOCKED | RELEASED
	AmountCents int64  `json:"amountCents"`
	EscrowRef   string `json:"escrowRef"`
	CreatedAt   string `json:"createdAt"`
}

// ToAuctionResp maps an AuctionView (auction + server-computed price) to its API
// response.
func ToAuctionResp(v biz.AuctionView) AuctionResp {
	a := v.Auction

	resp := AuctionResp{
		ID:                  a.ID.String(),
		LotID:               a.LotID.String(),
		State:               string(a.State),
		CeilingCents:        a.CeilingCents,
		FloorCents:          a.FloorCents,
		DropStepCents:       a.DropStepCents,
		DropIntervalSeconds: a.DropIntervalSeconds,
		CurrentPriceCents:   v.CurrentPrice,
		HammerPriceCents:    a.HammerPriceCents,
		CreatedAt:           isoTime(a.CreatedAt),
	}

	if a.OpenAt != nil {
		resp.OpenAt = isoTime(*a.OpenAt)
	}

	if v.NextDropAt != nil {
		resp.NextDropAt = isoTime(*v.NextDropAt)
	}

	if a.HammerAt != nil {
		resp.HammerAt = isoTime(*a.HammerAt)
	}

	if a.WinnerAccountID != nil {
		resp.WinnerAccountID = a.WinnerAccountID.String()
	}

	return resp
}

// ToBareAuctionResp maps a raw entity.Auction (no server price view) for admin
// transition responses, computing the price at `now`.
func ToBareAuctionResp(a entity.Auction, now time.Time) AuctionResp {
	return ToAuctionResp(biz.AuctionView{
		Auction:      a,
		CurrentPrice: biz.CurrentPrice(a, now),
		NextDropAt:   biz.NextDropAt(a, now),
	})
}

// ToReservationResp maps an entity.Reservation to its API response.
func ToReservationResp(r entity.Reservation) ReservationResp {
	return ReservationResp{
		ID:          r.ID.String(),
		AuctionID:   r.AuctionID.String(),
		AccountID:   r.AccountID.String(),
		Kind:        string(r.Kind),
		State:       string(r.State),
		AmountCents: r.AmountCents,
		EscrowRef:   r.EscrowRef,
		CreatedAt:   isoTime(r.CreatedAt),
	}
}
