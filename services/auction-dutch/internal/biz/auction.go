package biz

import (
	"application/internal/entity"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// reservationDepositBips is the 10% reservation deposit, in basis points of the
// auction ceiling (root CLAUDE.md §4: "10% reservation deposit").
const reservationDepositBips = 1000 // 10.00%

type auction struct {
	logger *slog.Logger
	repo   RepositoryAuction
	clock  Clock
}

var _ UsecaseAuction = (*auction)(nil)

// NewAuction constructs the Dutch auction use case. The clock is injected so the
// server-authoritative price + hammer decisions are deterministic in tests.
func NewAuction(logger *slog.Logger, repo RepositoryAuction, clock Clock) *auction {
	return &auction{
		logger: logger.With("layer", "AuctionUsecase"),
		repo:   repo,
		clock:  clock,
	}
}

// Get returns the auction plus its server-computed current price + next drop at
// the read clock (public read).
func (uc *auction) Get(ctx context.Context, id uuid.UUID) (AuctionView, error) {
	a, err := uc.repo.Get(ctx, id)
	if err != nil {
		return AuctionView{}, err
	}

	now := uc.clock.Now()

	return AuctionView{
		Auction:      a,
		CurrentPrice: CurrentPrice(a, now),
		NextDropAt:   NextDropAt(a, now),
	}, nil
}

// Reserve records a participant's 10% reservation deposit request.
func (uc *auction) Reserve(ctx context.Context, auctionID uuid.UUID, in ReserveInput) (entity.Reservation, error) {
	return uc.requestLock(ctx, auctionID, in, entity.KindDeposit10)
}

// Lock records a participant's 100% full-lock request before open.
func (uc *auction) Lock(ctx context.Context, auctionID uuid.UUID, in ReserveInput) (entity.Reservation, error) {
	return uc.requestLock(ctx, auctionID, in, entity.KindFullLock)
}

// requestLock is the shared body of Reserve/Lock: it validates the input,
// computes the amount for the kind, upserts the participant with its cached
// eligibility, inserts a REQUESTED reservation, and emits escrow.lock_requested —
// all atomically via the repo. Idempotent per (auction, account, kind).
func (uc *auction) requestLock(
	ctx context.Context,
	auctionID uuid.UUID,
	in ReserveInput,
	kind entity.ReservationKind,
) (entity.Reservation, error) {
	logger := uc.logger.With("method", "requestLock", "auction", auctionID, "account", in.AccountID, "kind", kind)

	if in.AccountID == uuid.Nil {
		return entity.Reservation{}, fmt.Errorf("%w: missing account", ErrResourceInvalid)
	}

	if !in.Tier.Eligible() {
		return entity.Reservation{}, fmt.Errorf("%w: tier %q cannot participate", ErrResourceInvalid, in.Tier)
	}

	if !in.KycApproved {
		return entity.Reservation{}, fmt.Errorf("%w: KYC not approved", ErrResourceInvalid)
	}

	a, err := uc.repo.Get(ctx, auctionID)
	if err != nil {
		return entity.Reservation{}, err
	}

	// Locks are only taken before the room opens (SCHEDULED). Once OPEN or beyond,
	// the participation set is frozen.
	if a.State != entity.AuctionScheduled {
		return entity.Reservation{}, fmt.Errorf("%w: auction is %s, not SCHEDULED", ErrResourceInvalid, a.State)
	}

	amount := uc.amountForKind(a, kind)

	// escrow_ref is producer-stable per (auction, account, kind) so a retried
	// reserve/lock dedups in the outbox AND lets escrow echo it back idempotently.
	escrowRef := fmt.Sprintf("auction-dutch:lock:%s:%s:%s", auctionID, in.AccountID, kind)

	res := entity.Reservation{
		ID:          uuid.New(),
		AuctionID:   auctionID,
		AccountID:   in.AccountID,
		Kind:        kind,
		AmountCents: amount,
		State:       entity.ReservationRequested,
		EscrowRef:   escrowRef,
		CreatedAt:   uc.clock.Now(),
	}

	part := entity.Participant{
		AuctionID:     auctionID,
		AccountID:     in.AccountID,
		KycApproved:   in.KycApproved,
		Tier:          in.Tier,
		ReservationSt: entity.ReservationRequested,
		FullLockState: entity.ReservationRequested,
		JoinedAt:      uc.clock.Now(),
	}

	outbox, err := newEscrowLockRequestedOutbox(res)
	if err != nil {
		return entity.Reservation{}, err
	}

	saved, err := uc.repo.ReserveTx(ctx, part, res, outbox)
	if err != nil {
		return entity.Reservation{}, err
	}

	logger.InfoContext(ctx, "lock requested", "amount_cents", amount, "escrow_ref", escrowRef)

	return saved, nil
}

// amountForKind returns the USDC-cents amount a reservation kind locks: 10% of
// the ceiling for the deposit, the full ceiling for the full lock.
func (uc *auction) amountForKind(a entity.Auction, kind entity.ReservationKind) int64 {
	if kind == entity.KindFullLock {
		return a.CeilingCents
	}

	return a.CeilingCents * reservationDepositBips / 10000 //nolint:mnd // bips denominator
}

// Buy is THE hammer action (root CLAUDE.md §3). It re-computes the price
// server-side (the client's view is advisory), validates the auction is OPEN and
// the caller is fully eligible, then atomically transitions OPEN -> HAMMER.
func (uc *auction) Buy(ctx context.Context, auctionID, accountID uuid.UUID) (entity.Auction, error) {
	logger := uc.logger.With("method", "Buy", "auction", auctionID, "account", accountID)

	a, err := uc.repo.Get(ctx, auctionID)
	if err != nil {
		return entity.Auction{}, err
	}

	if a.State != entity.AuctionOpen {
		logger.WarnContext(ctx, "buy on non-open auction", "state", a.State)

		return entity.Auction{}, fmt.Errorf("%w: auction is %s, not OPEN", ErrResourceInvalid, a.State)
	}

	part, err := uc.repo.GetParticipant(ctx, auctionID, accountID)
	if err != nil {
		if errors.Is(err, ErrResourceNotFound) {
			return entity.Auction{}, fmt.Errorf("%w: not a participant", ErrResourceInvalid)
		}

		return entity.Auction{}, err
	}

	if !part.Eligible() {
		logger.WarnContext(ctx, "ineligible buy",
			"kyc", part.KycApproved, "tier", part.Tier,
			"deposit", part.ReservationSt, "full_lock", part.FullLockState)

		return entity.Auction{}, fmt.Errorf("%w: participant not eligible (kyc/tier/deposit/full_lock)", ErrResourceInvalid)
	}

	// Server-authoritative price: the hammer price is whatever the descending curve
	// reads NOW, never the client's submitted/expected price.
	now := uc.clock.Now()
	price := CurrentPrice(a, now)

	idempotencyKey := fmt.Sprintf("auction-dutch:hammer:%s", auctionID)

	outbox, err := newAuctionHammerOutbox(a, accountID, price, now, idempotencyKey)
	if err != nil {
		return entity.Auction{}, err
	}

	// First valid buy wins: the conditional UPDATE only fires while still OPEN.
	updated, err := uc.repo.HammerTx(ctx, auctionID, accountID, price, now, outbox)
	if err != nil {
		if errors.Is(err, ErrResourceInvalid) {
			logger.WarnContext(ctx, "buy lost the race (already hammered)")
		}

		return entity.Auction{}, err
	}

	logger.InfoContext(ctx, "hammer", "winner", accountID, "price_cents", price)

	return updated, nil
}

// Open transitions a SCHEDULED auction to OPEN (admin). It requires at least one
// fully-eligible participant (KYC ∧ tier ∧ both locks LOCKED) and sets open_at as
// the price clock origin. Emits auction.opened.
func (uc *auction) Open(ctx context.Context, id uuid.UUID) (entity.Auction, error) {
	logger := uc.logger.With("method", "Open", "auction", id)

	a, err := uc.repo.Get(ctx, id)
	if err != nil {
		return entity.Auction{}, err
	}

	if !a.State.CanOpen() {
		return entity.Auction{}, fmt.Errorf("%w: auction is %s, cannot open", ErrResourceInvalid, a.State)
	}

	eligible, err := uc.repo.CountEligibleParticipants(ctx, id)
	if err != nil {
		return entity.Auction{}, err
	}

	if eligible < 1 {
		logger.WarnContext(ctx, "open rejected: no fully-locked participant")

		return entity.Auction{}, fmt.Errorf("%w: no eligible (fully-locked) participant", ErrResourceInvalid)
	}

	openAt := uc.clock.Now()

	// Project the opened auction so the emitted event carries the clock origin.
	opened := a
	opened.State = entity.AuctionOpen
	opened.OpenAt = &openAt

	idempotencyKey := fmt.Sprintf("auction-dutch:open:%s", id)

	outbox, err := newAuctionOpenedOutbox(opened, idempotencyKey)
	if err != nil {
		return entity.Auction{}, err
	}

	updated, err := uc.repo.OpenTx(ctx, id, openAt, outbox)
	if err != nil {
		return entity.Auction{}, err
	}

	logger.InfoContext(ctx, "auction opened", "open_at", openAt)

	return updated, nil
}

// Complete transitions a SETTLING auction to COMPLETED (admin) and emits
// auction.completed.
func (uc *auction) Complete(ctx context.Context, id uuid.UUID) (entity.Auction, error) {
	return uc.adminTransition(ctx, id, func(s entity.AuctionState) bool { return s.CanComplete() },
		entity.AuctionSettling, entity.AuctionCompleted, "complete")
}

// Abort transitions a non-terminal pre-settlement auction to ABORTED (admin) and
// emits auction.completed {final_state: ABORTED}.
func (uc *auction) Abort(ctx context.Context, id uuid.UUID) (entity.Auction, error) {
	logger := uc.logger.With("method", "Abort", "auction", id)

	a, err := uc.repo.Get(ctx, id)
	if err != nil {
		return entity.Auction{}, err
	}

	if !a.State.CanAbort() {
		return entity.Auction{}, fmt.Errorf("%w: auction is %s, cannot abort", ErrResourceInvalid, a.State)
	}

	idempotencyKey := fmt.Sprintf("auction-dutch:abort:%s", id)

	outbox, err := newAuctionCompletedOutbox(a, entity.AuctionAborted, idempotencyKey)
	if err != nil {
		return entity.Auction{}, err
	}

	updated, err := uc.repo.TransitionTx(ctx, id, a.State, entity.AuctionAborted, outbox)
	if err != nil {
		return entity.Auction{}, err
	}

	logger.InfoContext(ctx, "auction aborted", "from", a.State)

	return updated, nil
}

// adminTransition is the shared body of Complete: load, gate the source state via
// `allowed`, emit auction.completed, and flip from->to atomically.
func (uc *auction) adminTransition(
	ctx context.Context,
	id uuid.UUID,
	allowed func(entity.AuctionState) bool,
	from, to entity.AuctionState,
	action string,
) (entity.Auction, error) {
	logger := uc.logger.With("method", "adminTransition", "auction", id, "action", action)

	a, err := uc.repo.Get(ctx, id)
	if err != nil {
		return entity.Auction{}, err
	}

	if !allowed(a.State) {
		return entity.Auction{}, fmt.Errorf("%w: auction is %s, cannot %s", ErrResourceInvalid, a.State, action)
	}

	idempotencyKey := fmt.Sprintf("auction-dutch:%s:%s", action, id)

	outbox, err := newAuctionCompletedOutbox(a, to, idempotencyKey)
	if err != nil {
		return entity.Auction{}, err
	}

	updated, err := uc.repo.TransitionTx(ctx, id, from, to, outbox)
	if err != nil {
		return entity.Auction{}, err
	}

	logger.InfoContext(ctx, "auction transitioned", "from", from, "to", to)

	return updated, nil
}

// CreateFromLotScheduled creates a SCHEDULED Dutch auction from a catalog
// lot.scheduled event. Only DUTCH lots are materialized; VICKREY/UNIQBID are a
// no-op success (owned by auction-passive). Idempotent on idempotencyKey.
func (uc *auction) CreateFromLotScheduled(ctx context.Context, in LotScheduledInput, idempotencyKey string) error {
	logger := uc.logger.With("method", "CreateFromLotScheduled", "lot", in.LotID, "mode", in.Mode)

	if !in.Mode.IsDutch() {
		logger.InfoContext(ctx, "ignoring non-DUTCH lot.scheduled")

		return nil
	}

	id := in.AuctionID
	if id == uuid.Nil {
		id = uuid.New()
	}

	// Seed the descending-price params from the lot's reserve. The ceiling defaults
	// to a multiple of the reserve floor; the house tunes drop_step/interval before
	// open (out of scope here — sane defaults keep the engine well-formed).
	floor := in.ReserveCents

	a := entity.Auction{
		ID:                  id,
		LotID:               in.LotID,
		State:               entity.AuctionScheduled,
		CeilingCents:        defaultCeiling(floor),
		FloorCents:          floor,
		DropStepCents:       defaultDropStep(floor),
		DropIntervalSeconds: defaultDropIntervalSeconds,
		CreatedAt:           uc.clock.Now(),
	}

	if err := uc.repo.CreateAuctionTx(ctx, a, idempotencyKey); err != nil {
		if errors.Is(err, ErrResourceExists) {
			logger.InfoContext(ctx, "duplicate lot.scheduled ignored", "key", idempotencyKey)

			return nil
		}

		return err
	}

	logger.InfoContext(ctx, "scheduled dutch auction created", "auction", a.ID, "floor_cents", floor)

	return nil
}

// ApplyEscrowLocked flips the matching REQUESTED reservation to LOCKED and updates
// the participant's lock flag, on an escrow.locked event. Idempotent on
// idempotencyKey; an unknown escrow_ref is a no-op success (it may belong to
// another service's trade).
func (uc *auction) ApplyEscrowLocked(ctx context.Context, in EscrowLockedInput, idempotencyKey string) error {
	logger := uc.logger.With("method", "ApplyEscrowLocked", "escrow_ref", in.EscrowRef)

	if in.EscrowRef == "" {
		logger.InfoContext(ctx, "escrow.locked without escrow_ref ignored")

		return nil
	}

	err := uc.repo.ApplyEscrowLockedTx(ctx, in.EscrowRef, idempotencyKey)
	switch {
	case errors.Is(err, ErrResourceExists):
		logger.InfoContext(ctx, "duplicate escrow.locked ignored", "key", idempotencyKey)

		return nil
	case errors.Is(err, ErrResourceNotFound):
		logger.InfoContext(ctx, "escrow.locked for unknown reservation ignored", "escrow_ref", in.EscrowRef)

		return nil
	case err != nil:
		return err
	}

	logger.InfoContext(ctx, "reservation locked by escrow", "escrow_ref", in.EscrowRef)

	return nil
}

// Default descending-price params used when materializing an auction from a lot.
// They keep the engine well-formed; the house re-tunes them before open.
const (
	defaultDropIntervalSeconds int64 = 30 // a drop every 30s
)

// defaultCeiling seeds the starting price at 2x the reserve floor (a sane,
// non-zero ceiling above the floor).
func defaultCeiling(floor int64) int64 {
	if floor <= 0 {
		return 0
	}

	return floor * 2 //nolint:mnd // 2x reserve as the opening ceiling
}

// defaultDropStep seeds the per-interval drop at 1% of the ceiling so the price
// reaches the floor in ~100 intervals.
func defaultDropStep(floor int64) int64 {
	step := defaultCeiling(floor) / 100 //nolint:mnd // 1% of ceiling per drop
	if step <= 0 {
		return 1
	}

	return step
}
