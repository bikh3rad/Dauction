package biz

import (
	"application/internal/entity"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type auction struct {
	logger  *slog.Logger
	repo    RepositoryAuction
	debitor CreditDebitor
}

var _ UsecaseAuction = (*auction)(nil)

// NewAuction constructs the passive-auction use case.
func NewAuction(logger *slog.Logger, repo RepositoryAuction, debitor CreditDebitor) *auction {
	return &auction{
		logger:  logger.With("layer", "AuctionUsecase"),
		repo:    repo,
		debitor: debitor,
	}
}

// Get returns public auction info plus its participant count.
func (uc *auction) Get(ctx context.Context, id uuid.UUID) (entity.Auction, int, error) {
	a, err := uc.repo.Get(ctx, id)
	if err != nil {
		return entity.Auction{}, 0, err
	}

	count, err := uc.repo.ParticipantCount(ctx, id)
	if err != nil {
		return entity.Auction{}, 0, err
	}

	return a, count, nil
}

// PlaceBid implements the bid-credit flow (CLAUDE.md §5): validate OPEN + window
// + price, debit one credit synchronously BEFORE persisting, then write the
// immutable bid row + bid.placed event in one tx.
//
// A debit is never burned without a recorded bid: the debit idempotency key is
// derived deterministically from (auction, bidder, price, requestId), so if the
// bid-write tx fails the caller retries with the SAME key and the bids service
// replays the debit as a no-op (no double burn).
func (uc *auction) PlaceBid(ctx context.Context, in PlaceBidInput) (entity.PassiveBid, error) {
	logger := uc.logger.With("method", "PlaceBid", "auction", in.AuctionID, "bidder", in.BidderID)

	if in.PriceCents <= 0 {
		return entity.PassiveBid{}, fmt.Errorf("%w: price must be positive", ErrResourceInvalid)
	}

	a, err := uc.repo.Get(ctx, in.AuctionID)
	if err != nil {
		return entity.PassiveBid{}, err
	}

	if !a.State.AcceptsBids() {
		return entity.PassiveBid{}, fmt.Errorf("%w: auction is %s, not OPEN", ErrResourceInvalid, a.State)
	}

	if !time.Now().UTC().Before(a.ClosesAt) {
		return entity.PassiveBid{}, fmt.Errorf("%w: auction window has closed", ErrResourceInvalid)
	}

	// VICKREY: exactly one sealed bid per bidder — reject a second up front (the
	// DB unique index is the backstop).
	vickrey := a.Atype == entity.ModeVickrey
	if vickrey {
		has, hErr := uc.repo.HasBid(ctx, in.AuctionID, in.BidderID)
		if hErr != nil {
			return entity.PassiveBid{}, hErr
		}

		if has {
			return entity.PassiveBid{}, fmt.Errorf("%w: VICKREY allows one sealed bid per bidder", ErrResourceInvalid)
		}
	}

	// Deterministic debit key: a retry of the SAME logical bid replays the debit
	// (no double-burn); a different price/request is a distinct debit.
	debitKey := debitIdempotencyKey(in.AuctionID, in.BidderID, in.PriceCents, in.RequestID)

	// Step 1: debit one credit (synchronous, authoritative). Out of credits ->
	// ErrResourceInvalid before we persist anything.
	if err := uc.debitor.Debit(ctx, in.BidderID, 1, debitKey, in.AuctionID); err != nil {
		logger.WarnContext(ctx, "credit debit failed", "error", err)

		return entity.PassiveBid{}, err
	}

	now := time.Now().UTC()
	bid := entity.PassiveBid{
		ID:                  uuid.New(),
		AuctionID:           in.AuctionID,
		BidderAccountID:     in.BidderID,
		PriceCents:          in.PriceCents,
		PlacedAt:            now,
		DebitIdempotencyKey: debitKey,
		CreatedAt:           now,
	}

	outbox, err := newBidPlacedOutbox(bid)
	if err != nil {
		return entity.PassiveBid{}, err
	}

	// Step 2: persist the bid + event atomically.
	if err := uc.repo.InsertBidTx(ctx, bid, vickrey, outbox); err != nil {
		if errors.Is(err, ErrResourceExists) {
			// A bid already exists for this (auction,bidder,price) or debit key — a
			// retry after a successful prior write. The credit was already accounted
			// for the existing bid; surface as invalid (duplicate) without burning.
			logger.InfoContext(ctx, "duplicate bid ignored", "key", debitKey)

			return entity.PassiveBid{}, fmt.Errorf("%w: duplicate bid", ErrResourceInvalid)
		}

		return entity.PassiveBid{}, err
	}

	logger.InfoContext(ctx, "bid placed", "bid", bid.ID, "price", in.PriceCents)

	return bid, nil
}

// Standing returns the caller's own sealed view. For UNIQBID each of the caller's
// prices is flagged with whether it is currently the lowest unique price across
// all bids (server-computed; never reveals other bidders' prices).
func (uc *auction) Standing(ctx context.Context, auctionID, bidderID uuid.UUID) (Standing, error) {
	a, err := uc.repo.Get(ctx, auctionID)
	if err != nil {
		return Standing{}, err
	}

	mine, err := uc.repo.BidsByBidder(ctx, auctionID, bidderID)
	if err != nil {
		return Standing{}, err
	}

	out := Standing{Auction: a, Prices: make([]StandingPrice, 0, len(mine))}

	var lowestUnique int64
	haveLowestUnique := false

	if a.Atype == entity.ModeUniqBid {
		all, aErr := uc.repo.BidsByAuction(ctx, auctionID)
		if aErr != nil {
			return Standing{}, aErr
		}

		if res, rErr := ResolveUniqBid(toBids(all)); rErr == nil && res.Won {
			lowestUnique = res.ClearedPriceCents
			haveLowestUnique = true
		}
	}

	for _, b := range mine {
		out.Prices = append(out.Prices, StandingPrice{
			PriceCents:     b.PriceCents,
			IsLowestUnique: haveLowestUnique && b.PriceCents == lowestUnique,
			PlacedAt:       b.PlacedAt,
		})
	}

	return out, nil
}

// Close runs the deterministic close & resolution (CLAUDE.md §3). OPEN -> CLOSING
// (emits auction.closed), resolve the immutable log, then CLOSING -> RESOLVED
// (winner, emits auction.won) or CLOSING -> ABORTED (UniqBid no-unique).
func (uc *auction) Close(ctx context.Context, auctionID uuid.UUID) (entity.Auction, error) {
	logger := uc.logger.With("method", "Close", "auction", auctionID)

	a, err := uc.repo.Get(ctx, auctionID)
	if err != nil {
		return entity.Auction{}, err
	}

	if !a.State.CanClose() {
		return entity.Auction{}, fmt.Errorf("%w: auction is %s, cannot close (must be OPEN)", ErrResourceInvalid, a.State)
	}

	closedAt := time.Now().UTC()

	closedOutbox, err := newAuctionClosedOutbox(a, closedAt)
	if err != nil {
		return entity.Auction{}, err
	}

	// OPEN -> CLOSING (conditional, emits auction.closed).
	closing, err := uc.repo.CloseTx(ctx, auctionID, closedOutbox)
	if err != nil {
		return entity.Auction{}, err
	}

	// Resolve from the immutable bid log (pure function).
	bids, err := uc.repo.BidsByAuction(ctx, auctionID)
	if err != nil {
		return entity.Auction{}, err
	}

	res, err := Resolve(closing.Atype, toBids(bids))
	if err != nil {
		return entity.Auction{}, err
	}

	var wonOutbox *entity.OutboxEvent
	if res.Won {
		ob, oErr := newAuctionWonOutbox(closing, res.WinnerAccountID, res.ClearedPriceCents)
		if oErr != nil {
			return entity.Auction{}, oErr
		}

		wonOutbox = &ob
	}

	// CLOSING -> RESOLVED (winner) or ABORTED (no winner); emits auction.won when won.
	resolved, err := uc.repo.ResolveTx(ctx, auctionID, res, wonOutbox)
	if err != nil {
		return entity.Auction{}, err
	}

	logger.InfoContext(ctx, "auction resolved", "won", res.Won, "winner", res.WinnerAccountID,
		"cleared", res.ClearedPriceCents, "state", resolved.State)

	return resolved, nil
}

// CreateFromLotScheduled creates an OPEN passive auction from a lot.scheduled
// event. Idempotent on idempotencyKey via the inbox; duplicates (or a lot that
// already has an auction) are no-op successes so replays never create a 2nd one.
func (uc *auction) CreateFromLotScheduled(ctx context.Context, in LotScheduledInput, idempotencyKey string) error {
	logger := uc.logger.With("method", "CreateFromLotScheduled", "lot", in.LotID)

	if !in.Mode.Valid() {
		return fmt.Errorf("%w: lot.scheduled mode %q is not a passive auction", ErrResourceInvalid, in.Mode)
	}

	if in.DurationDays <= 0 {
		return fmt.Errorf("%w: timed auction needs a positive duration", ErrResourceInvalid)
	}

	scheduledAt := in.ScheduledAt
	if scheduledAt.IsZero() {
		scheduledAt = time.Now().UTC()
	}

	closesAt := scheduledAt.Add(time.Duration(in.DurationDays) * 24 * time.Hour)

	a := entity.Auction{
		ID:           uuid.New(),
		LotID:        in.LotID,
		Atype:        in.Mode,
		State:        entity.StateOpen,
		ClosesAt:     closesAt,
		ReserveCents: in.ReserveCents,
		CreatedAt:    time.Now().UTC(),
	}

	if err := uc.repo.CreateAuctionTx(ctx, a, idempotencyKey); err != nil {
		if errors.Is(err, ErrResourceExists) {
			logger.InfoContext(ctx, "duplicate lot.scheduled ignored", "key", idempotencyKey)

			return nil
		}

		return err
	}

	logger.InfoContext(ctx, "passive auction opened", "auction", a.ID, "mode", in.Mode, "closesAt", closesAt)

	return nil
}

// toBids projects the immutable log onto the pure-resolution Bid shape.
func toBids(rows []entity.PassiveBid) []Bid {
	out := make([]Bid, 0, len(rows))
	for _, b := range rows {
		out = append(out, Bid{
			BidderAccountID: b.BidderAccountID,
			PriceCents:      b.PriceCents,
			PlacedAt:        b.PlacedAt,
		})
	}

	return out
}

// debitIdempotencyKey derives the deterministic per-bid debit key (CLAUDE.md §5):
// retries of the SAME logical bid replay the debit (no double-burn).
func debitIdempotencyKey(auctionID, bidderID uuid.UUID, priceCents int64, requestID string) string {
	return fmt.Sprintf("bid:%s:%s:%d:%s", auctionID, bidderID, priceCents, requestID)
}
