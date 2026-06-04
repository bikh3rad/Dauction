package dto

import (
	"application/internal/biz"
	"application/internal/entity"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// isoTime formats a time as ISO-8601 UTC, the API's only time encoding.
func isoTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// AuctionResp is the public, language-neutral auction read model (enum codes,
// integer cents, ISO-8601 UTC). It never exposes other bidders' prices. Winner /
// cleared price are present only after resolution.
type AuctionResp struct {
	ID                string `json:"id"`
	LotID             string `json:"lotId"`
	Atype             string `json:"atype"` // VICKREY | UNIQBID
	State             string `json:"state"`
	ClosesAt          string `json:"closesAt"` // ISO-8601 UTC
	ReserveCents      int64  `json:"reserveCents"`
	ParticipantCount  int    `json:"participantCount"`
	WinnerAccountID   string `json:"winnerAccountId,omitempty"`
	ClearedPriceCents int64  `json:"clearedPriceCents,omitempty"`
}

// PlaceBidRequest is the body of POST /apis/auctions/{id}/bid.
type PlaceBidRequest struct {
	PriceCents int64  `json:"priceCents" validate:"required,gt=0"`
	RequestID  string `json:"requestId"  validate:"omitempty,max=128"`
}

// Validate checks the place-bid request (no validator dependency in this module;
// the tags document the contract and this mirrors them). Failure -> a wrapped
// error mapped to RESOURCE_INVALID by dto.HandleError.
func (r PlaceBidRequest) Validate() error {
	if r.PriceCents <= 0 {
		return fmt.Errorf("priceCents must be a positive integer (USDC cents)")
	}

	if len(r.RequestID) > 128 {
		return fmt.Errorf("requestId too long")
	}

	return nil
}

// BidResp is the read model for an accepted bid. The price echoes the caller's
// own sealed bid (the caller already knows it); it is never broadcast to others.
type BidResp struct {
	ID         string `json:"id"`
	AuctionID  string `json:"auctionId"`
	PriceCents int64  `json:"priceCents"`
	PlacedAt   string `json:"placedAt"`
}

// StandingPriceResp is one of the caller's prices plus its lowest-unique flag
// (UNIQBID only). For VICKREY isLowestUnique is always false (the field is N/A).
type StandingPriceResp struct {
	PriceCents     int64  `json:"priceCents"`
	IsLowestUnique bool   `json:"isLowestUnique"`
	PlacedAt       string `json:"placedAt"`
}

// StandingResp is the caller's own sealed view of an auction (CLAUDE.md §6).
type StandingResp struct {
	AuctionID string              `json:"auctionId"`
	Atype     string              `json:"atype"`
	State     string              `json:"state"`
	ClosesAt  string              `json:"closesAt"`
	Prices    []StandingPriceResp `json:"prices"`
}

// ToAuctionResp maps an entity.Auction + participant count to its public response.
func ToAuctionResp(a entity.Auction, participants int) AuctionResp {
	resp := AuctionResp{
		ID:               a.ID.String(),
		LotID:            a.LotID.String(),
		Atype:            string(a.Atype),
		State:            string(a.State),
		ClosesAt:         isoTime(a.ClosesAt),
		ReserveCents:     a.ReserveCents,
		ParticipantCount: participants,
	}

	if a.WinnerAccountID != nil {
		resp.WinnerAccountID = a.WinnerAccountID.String()
		resp.ClearedPriceCents = a.ClearedPriceCents
	}

	return resp
}

// ToBidResp maps an accepted bid to its response.
func ToBidResp(b entity.PassiveBid) BidResp {
	return BidResp{
		ID:         b.ID.String(),
		AuctionID:  b.AuctionID.String(),
		PriceCents: b.PriceCents,
		PlacedAt:   isoTime(b.PlacedAt),
	}
}

// ToStandingResp maps the caller's sealed standing view to its response.
func ToStandingResp(s biz.Standing) StandingResp {
	out := StandingResp{
		AuctionID: s.Auction.ID.String(),
		Atype:     string(s.Auction.Atype),
		State:     string(s.Auction.State),
		ClosesAt:  isoTime(s.Auction.ClosesAt),
		Prices:    make([]StandingPriceResp, 0, len(s.Prices)),
	}

	for _, p := range s.Prices {
		out.Prices = append(out.Prices, StandingPriceResp{
			PriceCents:     p.PriceCents,
			IsLowestUnique: p.IsLowestUnique,
			PlacedAt:       isoTime(p.PlacedAt),
		})
	}

	return out
}

// callerID extracts the X-Account-Id header as a UUID. The gateway injects this
// after authN; an absent/invalid value is a client/auth error.
func CallerID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, fmt.Errorf("missing X-Account-Id")
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("X-Account-Id must be a UUID: %w", err)
	}

	return id, nil
}
