package biz

import (
	"application/internal/entity"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type dispute struct {
	logger *slog.Logger
	repo   RepositoryDispute
}

var _ UsecaseDispute = (*dispute)(nil)

// NewDispute constructs the dispute-court use case.
func NewDispute(logger *slog.Logger, repo RepositoryDispute) *dispute {
	return &dispute{
		logger: logger.With("layer", "DisputeUsecase"),
		repo:   repo,
	}
}

// Open creates an OPEN dispute and emits dispute.opened (escrow consumes it to
// suspend release: HELD -> DISPUTED). Only one non-terminal dispute may exist per
// trade — a second open while one is OPEN/UNDER_REVIEW is rejected with
// ErrResourceExists (enforced in the repo via a partial unique index + the
// CreateTx pre-check, both belt and suspenders).
func (uc *dispute) Open(ctx context.Context, p OpenParams) (entity.Dispute, error) {
	logger := uc.logger.With("method", "Open", "trade", p.TradeID)

	if !p.ReasonCode.Valid() {
		return entity.Dispute{}, fmt.Errorf("%w: unknown reason code %q", ErrResourceInvalid, p.ReasonCode)
	}

	if p.Claimant == uuid.Nil || p.Respondent == uuid.Nil || p.TradeID == "" {
		return entity.Dispute{}, fmt.Errorf("%w: claimant, respondent and tradeId are required", ErrResourceInvalid)
	}

	if p.Claimant == p.Respondent {
		return entity.Dispute{}, fmt.Errorf("%w: claimant and respondent must differ", ErrResourceInvalid)
	}

	now := time.Now().UTC()
	id := uuid.New()

	d := entity.Dispute{
		ID:                  id,
		TradeID:             p.TradeID,
		ClaimantAccountID:   p.Claimant,
		RespondentAccountID: p.Respondent,
		ReasonCode:          p.ReasonCode,
		State:               entity.StateOpen,
		EvidenceRef:         p.EvidenceRef,
		CreatedAt:           now,
	}

	audit := newAudit(id, p.Claimant, entity.ActionOpened, p.EvidenceRef, now)

	// producer-stable idempotency: one suspend-release signal per dispute.
	outbox, err := newOpenedOutbox(id, p.TradeID, p.Claimant, fmt.Sprintf("dispute:opened:%s", id))
	if err != nil {
		return entity.Dispute{}, err
	}

	created, err := uc.repo.CreateTx(ctx, d, audit, outbox)
	if err != nil {
		logger.WarnContext(ctx, "open dispute failed", "error", err)

		return entity.Dispute{}, err
	}

	logger.InfoContext(ctx, "dispute opened", "dispute", created.ID, "reason", created.ReasonCode)

	return created, nil
}

// Get returns the dispute + audit trail, gated to parties or admin.
func (uc *dispute) Get(ctx context.Context, tradeID string, caller uuid.UUID, admin bool) (DisputeView, error) {
	d, err := uc.repo.GetByTrade(ctx, tradeID)
	if err != nil {
		return DisputeView{}, err
	}

	if !admin && !d.IsParty(caller) {
		return DisputeView{}, fmt.Errorf("%w: only parties or admin may read a dispute", ErrResourceAccessDenied)
	}

	events, err := uc.repo.ListEvents(ctx, d.ID)
	if err != nil {
		return DisputeView{}, err
	}

	return DisputeView{Dispute: d, Events: events}, nil
}

// AddEvidence appends an EVIDENCE_ADDED audit row. Either party only; the dispute
// must still be non-terminal (no evidence after RESOLVED/WITHDRAWN).
func (uc *dispute) AddEvidence(ctx context.Context, tradeID string, caller uuid.UUID, detailRef string) error {
	logger := uc.logger.With("method", "AddEvidence", "trade", tradeID)

	d, err := uc.repo.GetByTrade(ctx, tradeID)
	if err != nil {
		return err
	}

	if !d.IsParty(caller) {
		return fmt.Errorf("%w: only parties may add evidence", ErrResourceAccessDenied)
	}

	if d.State.Terminal() {
		return fmt.Errorf("%w: cannot add evidence to a %s dispute", ErrResourceInvalid, d.State)
	}

	audit := newAudit(d.ID, caller, entity.ActionEvidenceAdded, detailRef, time.Now().UTC())
	if err := uc.repo.AppendEvent(ctx, audit); err != nil {
		return err
	}

	logger.InfoContext(ctx, "evidence added", "dispute", d.ID)

	return nil
}

// StartReview moves a dispute OPEN -> UNDER_REVIEW (admin/house). Illegal from any
// other state -> ErrResourceInvalid (CAS enforced in the repo).
func (uc *dispute) StartReview(ctx context.Context, disputeID uuid.UUID, admin uuid.UUID) (entity.Dispute, error) {
	logger := uc.logger.With("method", "StartReview", "dispute", disputeID)

	d, err := uc.repo.GetByID(ctx, disputeID)
	if err != nil {
		return entity.Dispute{}, err
	}

	if d.State != entity.StateOpen {
		return entity.Dispute{}, fmt.Errorf("%w: review only from OPEN (was %s)", ErrResourceInvalid, d.State)
	}

	audit := newAudit(disputeID, admin, entity.ActionReviewStarted, "", time.Now().UTC())

	updated, err := uc.repo.TransitionTx(ctx, disputeID, entity.StateOpen, entity.StateUnderReview, audit)
	if err != nil {
		logger.WarnContext(ctx, "start review failed", "error", err)

		return entity.Dispute{}, err
	}

	logger.InfoContext(ctx, "review started")

	return updated, nil
}

// Resolve sets the ruling and moves UNDER_REVIEW -> RESOLVED (admin), emitting
// dispute.resolved for escrow to execute. Design choice: resolution requires a
// prior REVIEW_STARTED (UNDER_REVIEW), so a dispute is always triaged before a
// verdict; resolving from OPEN -> ErrResourceInvalid. The ruling is immutable
// once set — a second resolve hits the CAS and returns ErrResourceInvalid.
func (uc *dispute) Resolve(ctx context.Context, tradeID string, ruling entity.Ruling, ruledBy uuid.UUID) (entity.Dispute, error) {
	logger := uc.logger.With("method", "Resolve", "trade", tradeID)

	if !ruling.Valid() {
		return entity.Dispute{}, fmt.Errorf("%w: unknown ruling %q", ErrResourceInvalid, ruling)
	}

	if ruledBy == uuid.Nil {
		return entity.Dispute{}, fmt.Errorf("%w: ruledBy is required", ErrResourceInvalid)
	}

	d, err := uc.repo.GetByTrade(ctx, tradeID)
	if err != nil {
		return entity.Dispute{}, err
	}

	if d.State != entity.StateUnderReview {
		return entity.Dispute{}, fmt.Errorf("%w: resolve only from UNDER_REVIEW (was %s)", ErrResourceInvalid, d.State)
	}

	now := time.Now().UTC()
	audit := newAudit(d.ID, ruledBy, entity.ActionRuled, string(ruling), now)

	outbox, err := newResolvedOutbox(d.ID, tradeID, ruling, fmt.Sprintf("dispute:resolved:%s", d.ID))
	if err != nil {
		return entity.Dispute{}, err
	}

	updated, err := uc.repo.ResolveTx(ctx, d.ID, entity.StateUnderReview, ruling, ruledBy, audit, outbox)
	if err != nil {
		logger.WarnContext(ctx, "resolve failed", "error", err)

		return entity.Dispute{}, err
	}

	logger.InfoContext(ctx, "dispute resolved", "ruling", ruling)

	return updated, nil
}

// Withdraw moves a dispute OPEN -> WITHDRAWN. Claimant only, OPEN state only
// (you cannot withdraw once the house has started reviewing).
func (uc *dispute) Withdraw(ctx context.Context, tradeID string, caller uuid.UUID) (entity.Dispute, error) {
	logger := uc.logger.With("method", "Withdraw", "trade", tradeID)

	d, err := uc.repo.GetByTrade(ctx, tradeID)
	if err != nil {
		return entity.Dispute{}, err
	}

	if d.ClaimantAccountID != caller {
		return entity.Dispute{}, fmt.Errorf("%w: only the claimant may withdraw", ErrResourceAccessDenied)
	}

	if d.State != entity.StateOpen {
		return entity.Dispute{}, fmt.Errorf("%w: withdraw only from OPEN (was %s)", ErrResourceInvalid, d.State)
	}

	audit := newAudit(d.ID, caller, entity.ActionWithdrawn, "", time.Now().UTC())

	updated, err := uc.repo.TransitionTx(ctx, d.ID, entity.StateOpen, entity.StateWithdrawn, audit)
	if err != nil {
		logger.WarnContext(ctx, "withdraw failed", "error", err)

		return entity.Dispute{}, err
	}

	logger.InfoContext(ctx, "dispute withdrawn")

	return updated, nil
}

// ListByState returns the admin queue, optionally filtered by state.
func (uc *dispute) ListByState(ctx context.Context, f ListFilter) ([]entity.Dispute, error) {
	if f.State != "" && !f.State.Valid() {
		return nil, fmt.Errorf("%w: unknown state filter %q", ErrResourceInvalid, f.State)
	}

	return uc.repo.ListByState(ctx, f.State)
}

// newAudit builds an immutable audit-trail row.
func newAudit(disputeID, actor uuid.UUID, action entity.Action, detailRef string, at time.Time) entity.DisputeEvent {
	return entity.DisputeEvent{
		ID:             uuid.New(),
		DisputeID:      disputeID,
		ActorAccountID: actor,
		Action:         action,
		DetailRef:      detailRef,
		CreatedAt:      at,
	}
}
