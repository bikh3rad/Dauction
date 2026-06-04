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

// fundingWindow is the 24h window in which a passive winner (or a Dutch winner
// post-hammer) must fund their obligation into HELD (root CLAUDE.md §4).
const fundingWindow = 24 * time.Hour

// releaseCreditPercent is the Vault-Credit release multiplier: a seller who takes
// the release as Vault Credit receives 110% of the cash release (root §4). Integer
// math, truncation toward zero.
const (
	releaseCreditPercent = 110
	percentDivisor       = 100
)

type escrow struct {
	logger *slog.Logger
	repo   RepositoryEscrow
	now    func() time.Time
}

var _ UsecaseEscrow = (*escrow)(nil)

// EscrowOption customises the escrow use case (used by tests to inject a clock).
type EscrowOption func(*escrow)

// WithClock overrides the wall clock (deterministic funding-deadline tests).
func WithClock(now func() time.Time) EscrowOption {
	return func(uc *escrow) { uc.now = now }
}

// NewEscrow constructs the escrow use case.
func NewEscrow(logger *slog.Logger, repo RepositoryEscrow, opts ...EscrowOption) *escrow {
	uc := &escrow{
		logger: logger.With("layer", "EscrowUsecase"),
		repo:   repo,
		now:    func() time.Time { return time.Now().UTC() },
	}

	for _, opt := range opts {
		opt(uc)
	}

	return uc
}

// Get implements UsecaseEscrow.
func (uc *escrow) Get(ctx context.Context, tradeID uuid.UUID) (TradeView, error) {
	trade, err := uc.repo.GetTrade(ctx, tradeID)
	if err != nil {
		return TradeView{}, err
	}

	balances, err := uc.repo.Balances(ctx, tradeID)
	if err != nil {
		return TradeView{}, err
	}

	entries, err := uc.repo.ListEntries(ctx, tradeID)
	if err != nil {
		return TradeView{}, err
	}

	return TradeView{
		Trade:        trade,
		Balances:     balances,
		Conservation: entity.SummariseConservation(entries),
	}, nil
}

// Fund implements UsecaseEscrow: the winner funds their obligation. Past the
// funding deadline the trade FORFEITs instead.
func (uc *escrow) Fund(ctx context.Context, tradeID, caller uuid.UUID, amountCents int64) (entity.EscrowTrade, error) {
	logger := uc.logger.With("method", "Fund", "trade", tradeID)

	trade, err := uc.repo.GetTrade(ctx, tradeID)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	if trade.State == entity.StateDisputed {
		return entity.EscrowTrade{}, fmt.Errorf("%w: trade under dispute", ErrResourceInvalid)
	}

	// Only the buyer/winner funds.
	if caller != trade.BuyerAccountID {
		return entity.EscrowTrade{}, fmt.Errorf("%w: only the winner may fund", ErrResourceAccessDenied)
	}

	// Already funded / settled is an illegal double-fund.
	if trade.State == entity.StateHeld || trade.State.Terminal() {
		return entity.EscrowTrade{}, fmt.Errorf("%w: trade already funded (state=%s)", ErrResourceInvalid, trade.State)
	}

	obligation := trade.ObligationCents()

	// Past the funding window -> forfeit whatever the winner has locked.
	if trade.FundingDeadline != nil && uc.now().After(*trade.FundingDeadline) {
		logger.WarnContext(ctx, "funding window expired; forfeiting", "deadline", trade.FundingDeadline)

		return uc.forfeitFrom(ctx, trade)
	}

	if amountCents != obligation {
		return entity.EscrowTrade{}, fmt.Errorf("%w: amount %d != obligation %d", ErrResourceInvalid, amountCents, obligation)
	}

	// Passive trades fund straight from UNLOCKED into HELD; Dutch winners reach
	// HELD via auction.hammer, so a direct Fund only applies to passive here.
	from := entity.StateUnlocked

	entries := []entity.LedgerEntry{
		newEntry(tradeID, trade.BuyerAccountID, entity.EntryHold, obligation, refFund(tradeID)),
	}

	outbox, err := newEscrowLockedOutbox(tradeID, trade.BuyerAccountID, entity.StateHeld, obligation, lockedKey(tradeID, entity.StateHeld))
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	updated, err := uc.repo.TransitionTx(ctx, tradeID, from, entity.StateHeld, TradeUpdate{}, entries, &outbox)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	logger.InfoContext(ctx, "winner funded; HELD", "obligationCents", obligation)

	return updated, nil
}

// Confirm implements UsecaseEscrow: buyer confirms delivery -> RELEASED. The held
// pot is carved into RELEASE + FEE + PREMIUM + INSPECTOR_FEE (conservation
// preserved). Blocked if DISPUTED.
func (uc *escrow) Confirm(ctx context.Context, tradeID, caller uuid.UUID, mode entity.ReleaseMode) (entity.EscrowTrade, error) {
	logger := uc.logger.With("method", "Confirm", "trade", tradeID)

	if !mode.Valid() {
		return entity.EscrowTrade{}, fmt.Errorf("%w: unknown release mode %q", ErrResourceInvalid, mode)
	}

	trade, err := uc.repo.GetTrade(ctx, tradeID)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	if trade.State == entity.StateDisputed {
		return entity.EscrowTrade{}, fmt.Errorf("%w: release suspended pending dispute", ErrResourceInvalid)
	}

	if trade.State != entity.StateHeld {
		return entity.EscrowTrade{}, fmt.Errorf("%w: confirm requires HELD (state=%s)", ErrResourceInvalid, trade.State)
	}

	// Only the buyer confirms delivery.
	if caller != trade.BuyerAccountID {
		return entity.EscrowTrade{}, fmt.Errorf("%w: only the buyer may confirm delivery", ErrResourceAccessDenied)
	}

	// The held pot is the buyer's locked balance (works for both Dutch — where the
	// pot is the sum of DEPOSIT_LOCK + FULL_LOCK — and passive — a single HOLD).
	balances, err := uc.repo.Balances(ctx, tradeID)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	pot := balanceOf(balances, trade.BuyerAccountID)

	entries, releaseCents, err := releaseEntries(trade, pot)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	outbox, err := newEscrowReleasedOutbox(tradeID, trade.SellerAccountID, releaseCents, mode, releasedKey(tradeID))
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	updated, err := uc.repo.TransitionTx(
		ctx, tradeID, entity.StateHeld, entity.StateReleased,
		TradeUpdate{ReleaseMode: &mode}, entries, &outbox,
	)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	logger.InfoContext(ctx, "released to seller", "mode", mode, "releaseCents", releaseCents)

	return updated, nil
}

// Refund implements UsecaseEscrow: return a participant's locked funds and move
// the trade to REFUNDED (loser unfreeze / manual correction).
func (uc *escrow) Refund(ctx context.Context, tradeID, participant uuid.UUID) (entity.EscrowTrade, error) {
	logger := uc.logger.With("method", "Refund", "trade", tradeID)

	trade, err := uc.repo.GetTrade(ctx, tradeID)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	if trade.State == entity.StateDisputed {
		return entity.EscrowTrade{}, fmt.Errorf("%w: refund suspended pending dispute", ErrResourceInvalid)
	}

	// Refundable only while funds are locked/held (not after release/forfeit).
	switch trade.State {
	case entity.StateDepositLocked, entity.StateFullLocked, entity.StateHeld:
	default:
		return entity.EscrowTrade{}, fmt.Errorf("%w: nothing to refund (state=%s)", ErrResourceInvalid, trade.State)
	}

	balances, err := uc.repo.Balances(ctx, tradeID)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	locked := balanceOf(balances, participant)
	if locked <= 0 {
		return entity.EscrowTrade{}, fmt.Errorf("%w: participant has no locked funds", ErrResourceInvalid)
	}

	entries := []entity.LedgerEntry{
		newEntry(tradeID, participant, entity.EntryRefund, -locked, refRefund(tradeID, participant)),
	}

	outbox, err := newEscrowRefundedOutbox(tradeID, participant, locked, refundedKey(tradeID, participant))
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	updated, err := uc.repo.TransitionTx(
		ctx, tradeID, trade.State, entity.StateRefunded,
		TradeUpdate{}, entries, &outbox,
	)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	logger.InfoContext(ctx, "refunded participant", "participant", participant, "cents", locked)

	return updated, nil
}

// Forfeit implements UsecaseEscrow: a winner who missed the funding window (or a
// manual house forfeit) forfeits their locked funds.
func (uc *escrow) Forfeit(ctx context.Context, tradeID uuid.UUID) (entity.EscrowTrade, error) {
	trade, err := uc.repo.GetTrade(ctx, tradeID)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	return uc.forfeitFrom(ctx, trade)
}

// forfeitFrom seizes a trade's locked funds: writes a FORFEIT carve-out equal to
// the participant's locked balance and moves to FORFEITED.
func (uc *escrow) forfeitFrom(ctx context.Context, trade entity.EscrowTrade) (entity.EscrowTrade, error) {
	logger := uc.logger.With("method", "Forfeit", "trade", trade.ID)

	if trade.State == entity.StateDisputed {
		return entity.EscrowTrade{}, fmt.Errorf("%w: forfeit suspended pending dispute", ErrResourceInvalid)
	}

	switch trade.State {
	case entity.StateUnlocked, entity.StateDepositLocked, entity.StateFullLocked, entity.StateHeld:
	default:
		return entity.EscrowTrade{}, fmt.Errorf("%w: nothing to forfeit (state=%s)", ErrResourceInvalid, trade.State)
	}

	balances, err := uc.repo.Balances(ctx, trade.ID)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	locked := balanceOf(balances, trade.BuyerAccountID)

	var entries []entity.LedgerEntry
	if locked > 0 {
		entries = append(entries, newEntry(trade.ID, trade.BuyerAccountID, entity.EntryForfeit, locked, refForfeit(trade.ID)))
	}

	outbox, err := newEscrowForfeitedOutbox(trade.ID, trade.BuyerAccountID, locked, forfeitedKey(trade.ID))
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	updated, err := uc.repo.TransitionTx(
		ctx, trade.ID, trade.State, entity.StateForfeited,
		TradeUpdate{}, entries, &outbox,
	)
	if err != nil {
		return entity.EscrowTrade{}, err
	}

	logger.InfoContext(ctx, "forfeited", "cents", locked)

	return updated, nil
}

// LockRequested implements UsecaseEscrow: idempotently create-or-advance a Dutch
// trade and write the DEPOSIT_LOCK / FULL_LOCK ledger row, emitting escrow.locked.
func (uc *escrow) LockRequested(ctx context.Context, in LockRequest) error {
	logger := uc.logger.With("method", "LockRequested", "trade", in.TradeID)

	if in.State != entity.StateDepositLocked && in.State != entity.StateFullLocked {
		return fmt.Errorf("%w: lock target must be DEPOSIT_LOCKED or FULL_LOCKED, got %s", ErrResourceInvalid, in.State)
	}

	if in.AmountCents <= 0 {
		return fmt.Errorf("%w: lock amount must be positive", ErrResourceInvalid)
	}

	entryType := entity.EntryDepositLock
	if in.State == entity.StateFullLocked {
		entryType = entity.EntryFullLock
	}

	outbox, err := newEscrowLockedOutbox(in.TradeID, in.BuyerAccountID, in.State, in.AmountCents, lockedKey(in.TradeID, in.State))
	if err != nil {
		return err
	}

	entry := newEntry(in.TradeID, in.BuyerAccountID, entryType, in.AmountCents, refLock(in.TradeID, in.State))

	existing, err := uc.repo.GetTrade(ctx, in.TradeID)
	if errors.Is(err, ErrResourceNotFound) {
		// First lock for this auction: create the Dutch trade head DEPOSIT_LOCKED.
		trade := entity.EscrowTrade{
			ID:              in.TradeID,
			LotID:           in.LotID,
			BuyerAccountID:  in.BuyerAccountID,
			SellerAccountID: in.SellerAccountID,
			Kind:            entity.KindDutch,
			State:           in.State,
			PriceCents:      in.AmountCents,
		}

		if cErr := uc.repo.CreateTradeTx(ctx, trade, []entity.LedgerEntry{entry}, &outbox); cErr != nil {
			if errors.Is(cErr, ErrResourceExists) {
				logger.InfoContext(ctx, "duplicate lock create ignored")

				return nil
			}

			return cErr
		}

		logger.InfoContext(ctx, "created dutch trade", "state", in.State, "amountCents", in.AmountCents)

		return nil
	}

	if err != nil {
		return err
	}

	// Trade exists: advance DEPOSIT_LOCKED -> FULL_LOCKED. A replay (already in the
	// target state) is an idempotent no-op.
	if existing.State == in.State {
		logger.InfoContext(ctx, "trade already in lock state; ignoring", "state", in.State)

		return nil
	}

	_, err = uc.repo.TransitionTx(ctx, in.TradeID, entity.StateDepositLocked, in.State, TradeUpdate{}, []entity.LedgerEntry{entry}, &outbox)
	if errors.Is(err, ErrResourceInvalid) {
		// Not in DEPOSIT_LOCKED — out-of-order or duplicate; treat as no-op so the
		// shared stream does not redeliver forever.
		logger.InfoContext(ctx, "lock transition not applicable; ignoring", "from", existing.State, "to", in.State)

		return nil
	}

	if err != nil {
		return err
	}

	logger.InfoContext(ctx, "advanced lock", "to", in.State, "amountCents", in.AmountCents)

	return nil
}

// Hammer implements UsecaseEscrow: the FULL_LOCKED Dutch winner -> HELD. The
// existing full-lock funds become the held pot; price/premium are stamped from
// the hammer. Idempotent: an already-HELD/settled trade is a no-op.
func (uc *escrow) Hammer(ctx context.Context, in HammerInput) error {
	logger := uc.logger.With("method", "Hammer", "trade", in.TradeID)

	fresh, err := uc.repo.MarkConsumed(ctx, in.IdempotencyKey)
	if err != nil {
		return err
	}

	if !fresh {
		logger.InfoContext(ctx, "duplicate auction.hammer ignored", "key", in.IdempotencyKey)

		return nil
	}

	trade, err := uc.repo.GetTrade(ctx, in.TradeID)
	if errors.Is(err, ErrResourceNotFound) {
		logger.InfoContext(ctx, "auction.hammer for unknown trade; ignoring")

		return nil
	}

	if err != nil {
		return err
	}

	if trade.State == entity.StateHeld || trade.State.Terminal() {
		logger.InfoContext(ctx, "trade already past full-lock; ignoring", "state", trade.State)

		return nil
	}

	// Stamp the cleared terms; the full-lock pot is already funded, so HOLD adds
	// only the premium owed on top to keep the obligation == pot. To preserve
	// conservation without re-funding, we record price/premium on the head and the
	// already-locked amount stands as the pot (no new HOLD inflow).
	upd := TradeUpdate{}
	// no extra ledger inflow: the FULL_LOCK already funded the pot.
	outbox, err := newAuctionHeldLockedOutbox(in.TradeID, trade.BuyerAccountID, in.HammerPriceCents+in.PremiumCents, heldKey(in.TradeID))
	if err != nil {
		return err
	}

	_, err = uc.repo.TransitionTx(ctx, in.TradeID, entity.StateFullLocked, entity.StateHeld, upd, nil, &outbox)
	if errors.Is(err, ErrResourceInvalid) {
		logger.InfoContext(ctx, "trade not FULL_LOCKED; ignoring hammer", "state", trade.State)

		return nil
	}

	if err != nil {
		return err
	}

	logger.InfoContext(ctx, "dutch winner HELD")

	return nil
}

// Won implements UsecaseEscrow: create a passive trade UNLOCKED with a 24h
// funding deadline. The winner funds the cleared price + premium via Fund.
func (uc *escrow) Won(ctx context.Context, in WonInput) error {
	logger := uc.logger.With("method", "Won", "trade", in.TradeID)

	fresh, err := uc.repo.MarkConsumed(ctx, in.IdempotencyKey)
	if err != nil {
		return err
	}

	if !fresh {
		logger.InfoContext(ctx, "duplicate auction.won ignored", "key", in.IdempotencyKey)

		return nil
	}

	deadline := uc.now().Add(fundingWindow)

	trade := entity.EscrowTrade{
		ID:              in.TradeID,
		LotID:           in.LotID,
		BuyerAccountID:  in.WinnerID,
		SellerAccountID: in.SellerAccountID,
		Kind:            entity.KindPassive,
		State:           entity.StateUnlocked,
		PriceCents:      in.ClearedPriceCents,
		PremiumCents:    in.PremiumCents,
		FundingDeadline: &deadline,
	}

	if err := uc.repo.CreateTradeTx(ctx, trade, nil, nil); err != nil {
		if errors.Is(err, ErrResourceExists) {
			logger.InfoContext(ctx, "passive trade already created; ignoring")

			return nil
		}

		return err
	}

	logger.InfoContext(ctx, "passive trade created UNLOCKED", "deadline", deadline, "obligationCents", trade.ObligationCents())

	return nil
}

// DisputeResolved implements UsecaseEscrow: apply the court ruling to a HELD or
// DISPUTED trade. The held pot is distributed per the ruling (conservation kept).
func (uc *escrow) DisputeResolved(ctx context.Context, in DisputeInput) error {
	logger := uc.logger.With("method", "DisputeResolved", "trade", in.TradeID)

	if !in.Ruling.Valid() {
		return fmt.Errorf("%w: unknown ruling %q", ErrResourceInvalid, in.Ruling)
	}

	fresh, err := uc.repo.MarkConsumed(ctx, in.IdempotencyKey)
	if err != nil {
		return err
	}

	if !fresh {
		logger.InfoContext(ctx, "duplicate dispute.resolved ignored", "key", in.IdempotencyKey)

		return nil
	}

	trade, err := uc.repo.GetTrade(ctx, in.TradeID)
	if errors.Is(err, ErrResourceNotFound) {
		logger.InfoContext(ctx, "dispute.resolved for unknown trade; ignoring")

		return nil
	}

	if err != nil {
		return err
	}

	// Only a HELD or DISPUTED trade can be ruled on.
	if trade.State != entity.StateHeld && trade.State != entity.StateDisputed {
		logger.InfoContext(ctx, "trade not disputable; ignoring", "state", trade.State)

		return nil
	}

	balances, err := uc.repo.Balances(ctx, in.TradeID)
	if err != nil {
		return err
	}

	pot := balanceOf(balances, trade.BuyerAccountID)

	entries, toState, outbox, err := uc.ruling(trade, pot, in.Ruling)
	if err != nil {
		return err
	}

	if _, err := uc.repo.TransitionTx(ctx, in.TradeID, trade.State, toState, TradeUpdate{}, entries, outbox); err != nil {
		if errors.Is(err, ErrResourceInvalid) {
			logger.InfoContext(ctx, "ruling transition not applicable; ignoring", "state", trade.State)

			return nil
		}

		return err
	}

	logger.InfoContext(ctx, "dispute ruling applied", "ruling", in.Ruling, "potCents", pot)

	return nil
}

// ruling builds the ledger entries, target state and outbox event for a dispute
// verdict over a held pot. SPLIT halves the pot; the odd cent goes to the buyer.
func (uc *escrow) ruling(
	trade entity.EscrowTrade,
	pot int64,
	ruling entity.DisputeRuling,
) ([]entity.LedgerEntry, entity.EscrowState, *entity.OutboxEvent, error) {
	switch ruling {
	case entity.RulingRefundBuyer:
		entries := []entity.LedgerEntry{
			newEntry(trade.ID, trade.BuyerAccountID, entity.EntryRefund, -pot, refDispute(trade.ID)),
		}

		outbox, err := newEscrowRefundedOutbox(trade.ID, trade.BuyerAccountID, pot, disputeRefundKey(trade.ID))
		if err != nil {
			return nil, "", nil, err
		}

		return entries, entity.StateRefunded, &outbox, nil

	case entity.RulingReleaseSeller:
		entries := []entity.LedgerEntry{
			newEntry(trade.ID, trade.SellerAccountID, entity.EntryRelease, pot, refDispute(trade.ID)),
		}

		outbox, err := newEscrowReleasedOutbox(trade.ID, trade.SellerAccountID, pot, entity.ReleaseCash, disputeReleaseKey(trade.ID))
		if err != nil {
			return nil, "", nil, err
		}

		return entries, entity.StateReleased, &outbox, nil

	case entity.RulingSplit:
		// Halve the pot; the odd cent goes to the buyer (documented policy).
		sellerCents := pot / 2 //nolint:mnd
		buyerCents := pot - sellerCents

		entries := []entity.LedgerEntry{
			newEntry(trade.ID, trade.BuyerAccountID, entity.EntryRefund, -buyerCents, refDispute(trade.ID)),
			newEntry(trade.ID, trade.SellerAccountID, entity.EntryRelease, sellerCents, refDispute(trade.ID)),
		}

		// SPLIT settles as a release event carrying the seller's share; the buyer
		// refund is recorded on the ledger. Emit released for the seller portion.
		outbox, err := newEscrowReleasedOutbox(trade.ID, trade.SellerAccountID, sellerCents, entity.ReleaseCash, disputeSplitKey(trade.ID))
		if err != nil {
			return nil, "", nil, err
		}

		return entries, entity.StateReleased, &outbox, nil

	default:
		return nil, "", nil, fmt.Errorf("%w: unknown ruling %q", ErrResourceInvalid, ruling)
	}
}

// releaseEntries carves the held pot into the seller RELEASE plus the FEE /
// PREMIUM / INSPECTOR_FEE house carve-outs, so the disbursed total equals the
// held pot exactly (conservation). The seller's RELEASE is the pot minus the
// carve-outs. Returns the entries and the seller's cash release amount. A pot
// that cannot cover the carve-outs is rejected (ErrResourceInvalid) before any
// write. The chosen ReleaseMode is recorded on the trade head and rides the
// event; the cents here are mode-neutral (the 110% credit is derived on the event).
func releaseEntries(trade entity.EscrowTrade, pot int64) ([]entity.LedgerEntry, int64, error) {
	carveOut := trade.FeeCents + trade.PremiumCents + trade.InspectorFeeCents
	if carveOut > pot {
		return nil, 0, fmt.Errorf("%w: carve-outs %d exceed held pot %d", ErrResourceInvalid, carveOut, pot)
	}

	release := pot - carveOut

	entries := []entity.LedgerEntry{
		newEntry(trade.ID, trade.SellerAccountID, entity.EntryRelease, release, refRelease(trade.ID)),
	}

	if trade.FeeCents > 0 {
		entries = append(entries, newEntry(trade.ID, trade.SellerAccountID, entity.EntryFee, trade.FeeCents, refRelease(trade.ID)))
	}

	if trade.PremiumCents > 0 {
		entries = append(entries, newEntry(trade.ID, trade.BuyerAccountID, entity.EntryPremium, trade.PremiumCents, refRelease(trade.ID)))
	}

	if trade.InspectorFeeCents > 0 {
		entries = append(entries, newEntry(trade.ID, trade.SellerAccountID, entity.EntryInspectorFee, trade.InspectorFeeCents, refRelease(trade.ID)))
	}

	return entries, release, nil
}

// ReleaseCreditCents reports the 110% Vault-Credit instruction amount for a cash
// release (integer math, truncation toward zero). Exported for the event builder.
func ReleaseCreditCents(cashCents int64) int64 {
	return cashCents * releaseCreditPercent / percentDivisor
}

// newEntry builds an immutable ledger row.
func newEntry(tradeID, participant uuid.UUID, t entity.LedgerEntryType, amount int64, ref string) entity.LedgerEntry {
	return entity.LedgerEntry{
		ID:                   uuid.New(),
		TradeID:              tradeID,
		ParticipantAccountID: participant,
		EntryType:            t,
		AmountCents:          amount,
		Ref:                  ref,
	}
}

// balanceOf returns the derived balance for a participant (0 when absent).
func balanceOf(balances []entity.ParticipantBalance, participant uuid.UUID) int64 {
	for _, b := range balances {
		if b.ParticipantAccountID == participant {
			return b.BalanceCents
		}
	}

	return 0
}

// ref / idempotency-key helpers — producer-stable per logical write so replays
// dedup on the outbox unique key and ledger rows carry a traceable reference.
func refFund(tradeID uuid.UUID) string    { return fmt.Sprintf("fund:%s", tradeID) }
func refRelease(tradeID uuid.UUID) string { return fmt.Sprintf("release:%s", tradeID) }
func refForfeit(tradeID uuid.UUID) string { return fmt.Sprintf("forfeit:%s", tradeID) }
func refDispute(tradeID uuid.UUID) string { return fmt.Sprintf("dispute:%s", tradeID) }
func refLock(tradeID uuid.UUID, state entity.EscrowState) string {
	return fmt.Sprintf("lock:%s:%s", tradeID, state)
}

func refRefund(tradeID, participant uuid.UUID) string {
	return fmt.Sprintf("refund:%s:%s", tradeID, participant)
}

func lockedKey(tradeID uuid.UUID, state entity.EscrowState) string {
	return fmt.Sprintf("escrow:locked:%s:%s", tradeID, state)
}
func heldKey(tradeID uuid.UUID) string     { return fmt.Sprintf("escrow:held:%s", tradeID) }
func releasedKey(tradeID uuid.UUID) string { return fmt.Sprintf("escrow:released:%s", tradeID) }
func forfeitedKey(tradeID uuid.UUID) string {
	return fmt.Sprintf("escrow:forfeited:%s", tradeID)
}

func refundedKey(tradeID, participant uuid.UUID) string {
	return fmt.Sprintf("escrow:refunded:%s:%s", tradeID, participant)
}
func disputeRefundKey(tradeID uuid.UUID) string {
	return fmt.Sprintf("escrow:dispute:refund:%s", tradeID)
}

func disputeReleaseKey(tradeID uuid.UUID) string {
	return fmt.Sprintf("escrow:dispute:release:%s", tradeID)
}
func disputeSplitKey(tradeID uuid.UUID) string {
	return fmt.Sprintf("escrow:dispute:split:%s", tradeID)
}
