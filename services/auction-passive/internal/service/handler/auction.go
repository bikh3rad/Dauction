package handler

import (
	"application/internal/biz"
	"application/internal/service"
	"application/internal/service/dto"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type auctionHandler struct {
	logger  *slog.Logger
	mux     *http.ServeMux
	auction biz.UsecaseAuction
}

var _ service.Handler = (*auctionHandler)(nil)

// NewAuctionHandler constructs the auction-passive HTTP handler.
func NewAuctionHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseAuction) *auctionHandler {
	return &auctionHandler{
		logger:  logger.With("layer", "AuctionHandler"),
		mux:     mux,
		auction: uc,
	}
}

// RegisterHandler self-registers the routes (Go 1.22 method patterns). The caller
// account is read from the X-Account-Id header injected by the gateway.
func (h *auctionHandler) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("GET /apis/auctions/{id}", h.getAuction)
	h.mux.HandleFunc("POST /apis/auctions/{id}/bid", h.placeBid)
	h.mux.HandleFunc("GET /apis/auctions/{id}/standing", h.standing)
	h.mux.HandleFunc("POST /apis/admin/auctions/{id}/close", h.adminClose)

	return nil
}

// getAuction returns public auction info (state, closes_at, participant count).
//
//	@Summary		Auction info
//	@Description	Public passive-auction info: state, closes_at, participant count. Never exposes other bidders' prices (bids stay sealed until close).
//	@Tags			auction-passive
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		200	{object}	dto.AuctionResp
//	@Failure		400	{object}	dto.ErrorResponse	"Bad UUID"
//	@Failure		404	{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/auctions/{id} [get]
func (h *auctionHandler) getAuction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "GetAuction")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	a, count, err := h.auction.Get(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "get auction failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAuctionResp(a, count))
}

// placeBid places a sealed passive bid, spending one bid credit (CLAUDE.md §5).
//
//	@Summary		Place a sealed bid
//	@Description	Submit a sealed bid on a timed passive auction. Spends one bid credit (debited synchronously via the bids service BEFORE the bid is recorded). VICKREY allows one bid per bidder; UNIQBID allows many distinct prices. Out of credits / closed auction -> RESOURCE_INVALID. Emits bid.placed.
//	@Tags			auction-passive
//	@Accept			json
//	@Produce		json
//	@Param			X-Account-Id	header		string					true	"Caller account UUID (gateway-injected)"
//	@Param			id				path		string					true	"Auction UUID"
//	@Param			body			body		dto.PlaceBidRequest		true	"Bid"
//	@Success		201				{object}	dto.BidResp
//	@Failure		400				{object}	dto.ErrorResponse	"Bad request / out of credits / closed"
//	@Failure		401				{object}	dto.ErrorResponse	"Missing caller"
//	@Failure		404				{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/auctions/{id}/bid [post]
func (h *auctionHandler) placeBid(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "PlaceBid")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	caller, err := dto.CallerID(r.Header.Get("X-Account-Id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, err), w)

		return
	}

	var req dto.PlaceBidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	if err := req.Validate(); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	bid, err := h.auction.PlaceBid(ctx, biz.PlaceBidInput{
		AuctionID:  id,
		BidderID:   caller,
		PriceCents: req.PriceCents,
		RequestID:  req.RequestID,
	})
	if err != nil {
		logger.WarnContext(ctx, "place bid failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusCreated, dto.ToBidResp(bid))
}

// standing returns the caller's own sealed view of an auction.
//
//	@Summary		Your standing
//	@Description	The caller's own sealed view. VICKREY: your single sealed bid. UNIQBID: each of your distinct prices, flagged with whether it is currently the lowest unique price (server-computed). Never reveals other bidders' prices.
//	@Tags			auction-passive
//	@Produce		json
//	@Param			X-Account-Id	header		string	true	"Caller account UUID (gateway-injected)"
//	@Param			id				path		string	true	"Auction UUID"
//	@Success		200				{object}	dto.StandingResp
//	@Failure		400				{object}	dto.ErrorResponse	"Bad UUID"
//	@Failure		401				{object}	dto.ErrorResponse	"Missing caller"
//	@Failure		404				{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500				{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/auctions/{id}/standing [get]
func (h *auctionHandler) standing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Standing")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	caller, err := dto.CallerID(r.Header.Get("X-Account-Id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, err), w)

		return
	}

	s, err := h.auction.Standing(ctx, id, caller)
	if err != nil {
		logger.WarnContext(ctx, "standing failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToStandingResp(s))
}

// adminClose closes an OPEN auction and runs resolution (admin/house).
//
//	@Summary		Close & resolve an auction
//	@Description	House closes an OPEN passive auction: OPEN -> CLOSING -> runs the deterministic resolution (Vickrey 2nd-price / UniqBid lowest-unique) -> RESOLVED (winner) or ABORTED (UniqBid no-unique). Emits auction.closed then auction.won.
//	@Tags			auction-passive-admin
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		200	{object}	dto.AuctionResp
//	@Failure		400	{object}	dto.ErrorResponse	"Not OPEN"
//	@Failure		404	{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/auctions/{id}/close [post]
func (h *auctionHandler) adminClose(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "AdminClose")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	a, err := h.auction.Close(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "close auction failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	// Participant count is informational here; resolution already used the full log.
	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAuctionResp(a, 0))
}

// writeJSON encodes v as JSON with the given status, logging encode failures.
func writeJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}
