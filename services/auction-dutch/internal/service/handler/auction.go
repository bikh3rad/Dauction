package handler

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/service"
	"application/internal/service/dto"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Gateway-injected identity headers. The gateway authenticates the subject and
// fans in the caller's tier + KYC status from identity (root CLAUDE.md §6); this
// service trusts these headers rather than consuming kyc.approved /
// account.tier_changed itself — the documented eligibility approach (see CLAUDE.md).
const (
	headerAccountID   = "X-Account-Id"
	headerAccountTier = "X-Account-Tier"
	headerKycApproved = "X-Kyc-Approved"
)

type auctionHandler struct {
	logger  *slog.Logger
	mux     *http.ServeMux
	auction biz.UsecaseAuction
	clock   biz.Clock
}

var _ service.Handler = (*auctionHandler)(nil)

// NewAuctionHandler constructs the auction-dutch HTTP handler.
func NewAuctionHandler(logger *slog.Logger, mux *http.ServeMux, uc biz.UsecaseAuction, clock biz.Clock) *auctionHandler {
	return &auctionHandler{
		logger:  logger.With("layer", "AuctionHandler"),
		mux:     mux,
		auction: uc,
		clock:   clock,
	}
}

// RegisterHandler self-registers the auction routes (Go 1.22 method patterns).
func (h *auctionHandler) RegisterHandler(_ context.Context) error {
	// public read
	h.mux.HandleFunc("GET /apis/auctions/{id}", h.get)
	// participant actions
	h.mux.HandleFunc("POST /apis/auctions/{id}/reserve", h.reserve)
	h.mux.HandleFunc("POST /apis/auctions/{id}/lock", h.lock)
	h.mux.HandleFunc("POST /apis/auctions/{id}/buy", h.buy)
	// admin lifecycle
	h.mux.HandleFunc("POST /apis/admin/auctions/{id}/open", h.open)
	h.mux.HandleFunc("POST /apis/admin/auctions/{id}/complete", h.complete)
	h.mux.HandleFunc("POST /apis/admin/auctions/{id}/abort", h.abort)

	return nil
}

// get returns auction state + the server-computed current_price(now) + next_drop.
//
//	@Summary		Auction state + live price
//	@Description	Public read: the Dutch auction's state plus the SERVER-authoritative current price at now and the next drop instant. Clients render from these; the buy decision is always re-validated server-side.
//	@Tags			auction-dutch
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		200	{object}	dto.AuctionResp
//	@Failure		400	{object}	dto.ErrorResponse	"Bad UUID"
//	@Failure		404	{object}	dto.ErrorResponse	"Not Found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/auctions/{id} [get]
func (h *auctionHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Get")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	view, err := h.auction.Get(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "get auction failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAuctionResp(view))
}

// reserve records the participant's 10% reservation deposit request.
//
//	@Summary		Reserve (10% deposit)
//	@Description	Participant requests the 10% reservation deposit. Computes 10% of the ceiling, creates a REQUESTED reservation, and emits escrow.lock_requested {kind: DEPOSIT_10}. Caller identity + eligibility come from the gateway headers X-Account-Id / X-Account-Tier / X-Kyc-Approved.
//	@Tags			auction-dutch
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		201	{object}	dto.ReservationResp
//	@Failure		400	{object}	dto.ErrorResponse	"Bad UUID / ineligible / wrong state"
//	@Failure		401	{object}	dto.ErrorResponse	"Missing caller"
//	@Failure		404	{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/auctions/{id}/reserve [post]
func (h *auctionHandler) reserve(w http.ResponseWriter, r *http.Request) {
	h.lockRequest(w, r, "Reserve", h.auction.Reserve)
}

// lock records the participant's 100% full-lock request before open.
//
//	@Summary		Full lock (100%)
//	@Description	Participant requests the 100% full lock before open. Creates a REQUESTED reservation and emits escrow.lock_requested {kind: FULL_LOCK}. Both a LOCKED deposit and a LOCKED full lock are required to buy.
//	@Tags			auction-dutch
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		201	{object}	dto.ReservationResp
//	@Failure		400	{object}	dto.ErrorResponse	"Bad UUID / ineligible / wrong state"
//	@Failure		401	{object}	dto.ErrorResponse	"Missing caller"
//	@Failure		404	{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/auctions/{id}/lock [post]
func (h *auctionHandler) lock(w http.ResponseWriter, r *http.Request) {
	h.lockRequest(w, r, "Lock", h.auction.Lock)
}

// lockRequest is the shared body of reserve/lock: parse the auction id + caller
// eligibility headers, then call the supplied use-case method.
func (h *auctionHandler) lockRequest(
	w http.ResponseWriter,
	r *http.Request,
	method string,
	fn func(context.Context, uuid.UUID, biz.ReserveInput) (entity.Reservation, error),
) {
	ctx := r.Context()
	logger := h.logger.With("method", method)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	in, err := callerEligibility(r)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	res, err := fn(ctx, id, in)
	if err != nil {
		logger.WarnContext(ctx, "lock request failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusCreated, dto.ToReservationResp(res))
}

// buy is THE hammer action: re-validate server price + eligibility, transition
// OPEN -> HAMMER.
//
//	@Summary		Buy (hammer)
//	@Description	The hammer action. The server re-computes the current price (a stale client price is ignored), validates the auction is OPEN and the caller is a fully eligible participant (KYC ∧ tier ∈ {MEMBER,VIP} ∧ deposit LOCKED ∧ full_lock LOCKED), then atomically transitions OPEN -> HAMMER. The first valid buy wins; later buys return RESOURCE_INVALID. Emits auction.hammer.
//	@Tags			auction-dutch
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		200	{object}	dto.AuctionResp
//	@Failure		400	{object}	dto.ErrorResponse	"Not OPEN / ineligible / already hammered"
//	@Failure		401	{object}	dto.ErrorResponse	"Missing caller"
//	@Failure		404	{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/auctions/{id}/buy [post]
func (h *auctionHandler) buy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Buy")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	accountID, err := callerID(r)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	a, err := h.auction.Buy(ctx, id, accountID)
	if err != nil {
		logger.WarnContext(ctx, "buy failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToBareAuctionResp(a, h.clock.Now()))
}

// open transitions a SCHEDULED auction to OPEN (admin).
//
//	@Summary		Open auction (admin)
//	@Description	House opens a SCHEDULED auction. Requires at least one fully-locked, eligible participant. Sets open_at (the price clock origin) and emits auction.opened. Illegal transition -> RESOURCE_INVALID.
//	@Tags			auction-dutch-admin
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		200	{object}	dto.AuctionResp
//	@Failure		400	{object}	dto.ErrorResponse	"Illegal transition / no eligible participant"
//	@Failure		404	{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/auctions/{id}/open [post]
func (h *auctionHandler) open(w http.ResponseWriter, r *http.Request) {
	h.adminAction(w, r, "Open", h.auction.Open)
}

// complete transitions a SETTLING auction to COMPLETED (admin).
//
//	@Summary		Complete auction (admin)
//	@Description	House completes a SETTLING auction. Emits auction.completed {final_state: COMPLETED}. Illegal transition -> RESOURCE_INVALID.
//	@Tags			auction-dutch-admin
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		200	{object}	dto.AuctionResp
//	@Failure		400	{object}	dto.ErrorResponse	"Illegal transition"
//	@Failure		404	{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/auctions/{id}/complete [post]
func (h *auctionHandler) complete(w http.ResponseWriter, r *http.Request) {
	h.adminAction(w, r, "Complete", h.auction.Complete)
}

// abort transitions a non-terminal pre-settlement auction to ABORTED (admin).
//
//	@Summary		Abort auction (admin)
//	@Description	House aborts a non-terminal pre-settlement auction (e.g. threshold unmet). Emits auction.completed {final_state: ABORTED}. Illegal transition -> RESOURCE_INVALID.
//	@Tags			auction-dutch-admin
//	@Produce		json
//	@Param			id	path		string	true	"Auction UUID"
//	@Success		200	{object}	dto.AuctionResp
//	@Failure		400	{object}	dto.ErrorResponse	"Illegal transition"
//	@Failure		404	{object}	dto.ErrorResponse	"Auction not found"
//	@Failure		500	{object}	dto.ErrorResponse	"Internal Server Error"
//	@Router			/apis/admin/auctions/{id}/abort [post]
func (h *auctionHandler) abort(w http.ResponseWriter, r *http.Request) {
	h.adminAction(w, r, "Abort", h.auction.Abort)
}

// adminAction is the shared body of the admin lifecycle transitions.
func (h *auctionHandler) adminAction(
	w http.ResponseWriter,
	r *http.Request,
	method string,
	fn func(context.Context, uuid.UUID) (entity.Auction, error),
) {
	ctx := r.Context()
	logger := h.logger.With("method", method)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	a, err := fn(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "admin action failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToBareAuctionResp(a, h.clock.Now()))
}

// callerID extracts the authenticated subject from the gateway-injected
// X-Account-Id header, or ErrResourceAccessDenied when absent/malformed.
func callerID(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get(headerAccountID))
	if raw == "" {
		return uuid.Nil, errors.Join(biz.ErrResourceAccessDenied, errors.New("missing X-Account-Id"))
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, errors.Join(biz.ErrResourceAccessDenied, err)
	}

	return id, nil
}

// callerEligibility builds a biz.ReserveInput from the gateway-injected identity
// headers (subject + tier + KYC status).
func callerEligibility(r *http.Request) (biz.ReserveInput, error) {
	id, err := callerID(r)
	if err != nil {
		return biz.ReserveInput{}, err
	}

	tier := entity.Tier(strings.ToUpper(strings.TrimSpace(r.Header.Get(headerAccountTier))))
	kyc := strings.EqualFold(strings.TrimSpace(r.Header.Get(headerKycApproved)), "true")

	return biz.ReserveInput{
		AccountID:   id,
		Tier:        tier,
		KycApproved: kyc,
	}, nil
}

// writeJSON encodes v as JSON with the given status, logging encode failures.
func writeJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}
