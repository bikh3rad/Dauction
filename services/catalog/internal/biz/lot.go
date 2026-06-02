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

type lot struct {
	logger *slog.Logger
	repo   RepositoryLot
}

var _ UsecaseLot = (*lot)(nil)

// NewLot constructs the lot use case.
func NewLot(logger *slog.Logger, repo RepositoryLot) *lot {
	return &lot{
		logger: logger.With("layer", "LotUsecase"),
		repo:   repo,
	}
}

// GetWeekly returns this ISO week's SCHEDULED lots (public gallery read).
func (uc *lot) GetWeekly(ctx context.Context, week string) ([]entity.Lot, error) {
	if week == "" {
		week = ISOWeekOf(time.Now().UTC())
	}

	return uc.repo.GetWeekly(ctx, week)
}

// Get returns a lot plus its attestation summary, or ErrResourceNotFound.
func (uc *lot) Get(ctx context.Context, id uuid.UUID) (entity.Lot, []entity.Attestation, error) {
	l, err := uc.repo.Get(ctx, id)
	if err != nil {
		return entity.Lot{}, nil, err
	}

	atts, err := uc.repo.AttestationsByLot(ctx, id)
	if err != nil {
		return entity.Lot{}, nil, err
	}

	return l, atts, nil
}

// List returns lots matching the admin filter.
func (uc *lot) List(ctx context.Context, filter LotListFilter) ([]entity.Lot, error) {
	if filter.State != "" && !filter.State.Valid() {
		return nil, fmt.Errorf("%w: unknown state %q", ErrResourceInvalid, filter.State)
	}

	return uc.repo.List(ctx, filter)
}

// Attest records an inspector attestation on a lot and emits attestation.recorded.
// A FAIL on a DRAFT lot rejects the lot (terminal). The attestation row + the
// optional rejection + the outbox event commit in one transaction.
func (uc *lot) Attest(ctx context.Context, lotID uuid.UUID, in AttestInput) (entity.Attestation, error) {
	logger := uc.logger.With("method", "Attest", "lot", lotID)

	if !in.Result.Valid() {
		return entity.Attestation{}, fmt.Errorf("%w: unknown result %q", ErrResourceInvalid, in.Result)
	}

	l, err := uc.repo.Get(ctx, lotID)
	if err != nil {
		return entity.Attestation{}, err
	}

	// Attestations only make sense before the lot has been scheduled; a terminal
	// or already-listed lot rejects a new seal.
	if l.State == entity.LotScheduled || l.State == entity.LotRejected {
		return entity.Attestation{}, fmt.Errorf("%w: cannot attest a %s lot", ErrResourceInvalid, l.State)
	}

	att := entity.Attestation{
		ID:          uuid.New(),
		LotID:       lotID,
		InspectorID: in.InspectorID,
		Result:      in.Result,
		NotesRef:    in.NotesRef,
		RecordedAt:  time.Now().UTC(),
	}

	// A FAIL on a not-yet-certified lot is a rejection (terminal). A FAIL on a
	// CERTIFIED lot is recorded but does not retroactively pull it here.
	rejectLot := in.Result == entity.AttestFail && l.State == entity.LotDraft

	idempotencyKey := fmt.Sprintf("catalog:attestation:%s", att.ID)

	outbox, err := newAttestationRecordedOutbox(att.ID, lotID, in.InspectorID, in.Result == entity.AttestPass, idempotencyKey)
	if err != nil {
		return entity.Attestation{}, err
	}

	if err := uc.repo.RecordAttestationTx(ctx, att, rejectLot, outbox); err != nil {
		return entity.Attestation{}, err
	}

	logger.InfoContext(ctx, "attestation recorded", "result", in.Result, "rejected", rejectLot)

	return att, nil
}

// Certify moves a DRAFT lot to CERTIFIED. The certification gate requires an
// existing PASS attestation; without one the transition is illegal. Emits
// lot.certified. The state flip + outbox commit in one transaction.
func (uc *lot) Certify(ctx context.Context, lotID uuid.UUID) (entity.Lot, error) {
	logger := uc.logger.With("method", "Certify", "lot", lotID)

	l, err := uc.repo.Get(ctx, lotID)
	if err != nil {
		return entity.Lot{}, err
	}

	if !l.State.CanCertify() {
		logger.WarnContext(ctx, "illegal certify transition", "from", l.State)

		return entity.Lot{}, fmt.Errorf("%w: cannot certify a %s lot", ErrResourceInvalid, l.State)
	}

	// Certification gate: a PASS attestation must exist (CLAUDE.md §2).
	att, err := uc.latestPassAttestation(ctx, lotID)
	if err != nil {
		return entity.Lot{}, err
	}

	idempotencyKey := fmt.Sprintf("catalog:certify:%s", lotID)

	outbox, err := newLotCertifiedOutbox(lotID, att.ID, idempotencyKey)
	if err != nil {
		return entity.Lot{}, err
	}

	updated, err := uc.repo.CertifyTx(ctx, lotID, outbox)
	if err != nil {
		return entity.Lot{}, err
	}

	logger.InfoContext(ctx, "lot certified", "attestation", att.ID)

	return updated, nil
}

// Schedule moves a CERTIFIED lot to SCHEDULED for its ISO week, enforcing the
// weekly 32-cap atomically in the repo's conditional update. Emits lot.scheduled
// carrying the atype + params downstream auctions consume.
func (uc *lot) Schedule(ctx context.Context, lotID uuid.UUID, scheduledAt time.Time) (entity.Lot, error) {
	logger := uc.logger.With("method", "Schedule", "lot", lotID)

	l, err := uc.repo.Get(ctx, lotID)
	if err != nil {
		return entity.Lot{}, err
	}

	if !l.State.CanSchedule() {
		logger.WarnContext(ctx, "illegal schedule transition", "from", l.State)

		return entity.Lot{}, fmt.Errorf("%w: cannot schedule a %s lot (must be CERTIFIED)", ErrResourceInvalid, l.State)
	}

	when := scheduledAt.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}

	// Project the scheduled lot so the emitted event carries final params.
	scheduled := l
	scheduled.State = entity.LotScheduled
	scheduled.ScheduledAt = &when

	idempotencyKey := fmt.Sprintf("catalog:schedule:%s", lotID)

	outbox, err := newLotScheduledOutbox(scheduled, idempotencyKey)
	if err != nil {
		return entity.Lot{}, err
	}

	// The repo enforces the weekly cap in the same tx (conditional update gated on
	// the current SCHEDULED count for the week). Cap exceeded -> ErrResourceInvalid.
	updated, err := uc.repo.ScheduleTx(ctx, lotID, when, WeeklySupplyCap, outbox)
	if err != nil {
		if errors.Is(err, ErrResourceInvalid) {
			logger.WarnContext(ctx, "schedule rejected (cap reached or state changed)", "week", l.ISOWeek)
		}

		return entity.Lot{}, err
	}

	logger.InfoContext(ctx, "lot scheduled", "week", updated.ISOWeek, "at", when)

	return updated, nil
}

// CreateFromObjectListed creates a DRAFT lot from a vault object.listed event.
// Idempotent on idempotencyKey via the inbox; a duplicate event (or an object
// that already has a lot) is a no-op success so replays never create a 2nd lot.
func (uc *lot) CreateFromObjectListed(ctx context.Context, in ObjectListedInput, idempotencyKey string) error {
	logger := uc.logger.With("method", "CreateFromObjectListed", "object", in.ObjectID)

	if !in.Mode.Valid() {
		return fmt.Errorf("%w: unknown auction mode %q", ErrResourceInvalid, in.Mode)
	}

	// Timed auctions require a duration; DUTCH must not carry one.
	if in.Mode.Timed() && in.DurationDays == nil {
		return fmt.Errorf("%w: %s lot needs a duration", ErrResourceInvalid, in.Mode)
	}

	if !in.Mode.Timed() {
		in.DurationDays = nil
	}

	week := in.ISOWeek
	if week == "" {
		week = ISOWeekOf(time.Now().UTC())
	}

	l := entity.Lot{
		ID:                  uuid.New(),
		ObjectID:            in.ObjectID,
		SellerAccountID:     in.OwnerID,
		Mode:                in.Mode,
		DurationDays:        in.DurationDays,
		ReserveCents:        in.ReserveCents,
		AppraisedValueCents: in.AppraisedValueCents,
		State:               entity.LotDraft,
		ISOWeek:             week,
		CreatedAt:           time.Now().UTC(),
	}

	if err := uc.repo.CreateLotTx(ctx, l, idempotencyKey); err != nil {
		if errors.Is(err, ErrResourceExists) {
			logger.InfoContext(ctx, "duplicate object.listed ignored", "key", idempotencyKey)

			return nil
		}

		return err
	}

	logger.InfoContext(ctx, "draft lot created", "lot", l.ID, "mode", in.Mode)

	return nil
}

// latestPassAttestation returns a PASS attestation for the lot, or
// ErrResourceInvalid (the certification gate) when none exists.
func (uc *lot) latestPassAttestation(ctx context.Context, lotID uuid.UUID) (entity.Attestation, error) {
	atts, err := uc.repo.AttestationsByLot(ctx, lotID)
	if err != nil {
		return entity.Attestation{}, err
	}

	for _, a := range atts {
		if a.Result == entity.AttestPass {
			return a, nil
		}
	}

	return entity.Attestation{}, fmt.Errorf("%w: certification gate: no PASS attestation", ErrResourceInvalid)
}
