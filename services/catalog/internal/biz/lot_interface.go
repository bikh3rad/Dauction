package biz

import (
	"application/internal/entity"
	"context"
	"time"

	"github.com/google/uuid"
)

// WeeklySupplyCap is the maximum number of lots that may be SCHEDULED into a
// single ISO week's gallery (CLAUDE.md §2, §6 WEEKLY_CAP_REACHED).
const WeeklySupplyCap = 32

// AttestInput is the inspector's recorded judgement on a lot.
type AttestInput struct {
	InspectorID uuid.UUID
	Result      entity.AttestationResult
	NotesRef    string
}

// LotListFilter narrows the admin lot listing. Empty fields are ignored.
type LotListFilter struct {
	State   entity.LotState // optional state filter
	ISOWeek string          // optional ISO-week filter (e.g. "2026-W23")
}

// UsecaseLot is the lot use case consumed by handlers and the event consumer.
// It owns the certification gate and the weekly 32-cap (CLAUDE.md §2).
type UsecaseLot interface {
	// GetWeekly returns this ISO week's SCHEDULED lots (public gallery read).
	GetWeekly(ctx context.Context, week string) ([]entity.Lot, error)
	// Get returns a single lot by id with its attestation summary, or ErrResourceNotFound.
	Get(ctx context.Context, id uuid.UUID) (entity.Lot, []entity.Attestation, error)
	// List returns lots matching the admin filter (state/week).
	List(ctx context.Context, filter LotListFilter) ([]entity.Lot, error)

	// Attest records an inspector attestation on a lot and emits
	// attestation.recorded. A FAIL on a DRAFT lot moves it to REJECTED.
	Attest(ctx context.Context, lotID uuid.UUID, in AttestInput) (entity.Attestation, error)
	// Certify moves a DRAFT lot to CERTIFIED, requiring an existing PASS
	// attestation (certification gate). Emits lot.certified. Illegal -> ErrResourceInvalid.
	Certify(ctx context.Context, lotID uuid.UUID) (entity.Lot, error)
	// Schedule moves a CERTIFIED lot to SCHEDULED for its ISO week, enforcing the
	// weekly 32-cap in the same tx. Emits lot.scheduled. Cap exceeded / not
	// certified -> ErrResourceInvalid.
	Schedule(ctx context.Context, lotID uuid.UUID, scheduledAt time.Time) (entity.Lot, error)

	// CreateFromObjectListed creates a DRAFT lot from a vault object.listed event.
	// Idempotent on idempotencyKey via the inbox; a duplicate is a no-op success.
	CreateFromObjectListed(ctx context.Context, in ObjectListedInput, idempotencyKey string) error
}

// ObjectListedInput is the catalog-internal projection of a vault object.listed
// event used to seed a DRAFT lot.
type ObjectListedInput struct {
	ObjectID            uuid.UUID
	OwnerID             uuid.UUID
	Mode                entity.AuctionMode
	DurationDays        *int32 // nil for DUTCH
	ReserveCents        int64
	AppraisedValueCents int64
	ISOWeek             string
}

// RepositoryLot is the persistence seam (implemented by internal/repo, mocked in
// tests). State mutations that emit an event do so atomically with an outbox row.
type RepositoryLot interface {
	// GetWeekly returns SCHEDULED lots for the given ISO week.
	GetWeekly(ctx context.Context, week string) ([]entity.Lot, error)
	// Get returns the lot or ErrResourceNotFound.
	Get(ctx context.Context, id uuid.UUID) (entity.Lot, error)
	// List returns lots matching the filter.
	List(ctx context.Context, filter LotListFilter) ([]entity.Lot, error)

	// AttestationsByLot returns all attestations recorded against a lot.
	AttestationsByLot(ctx context.Context, lotID uuid.UUID) ([]entity.Attestation, error)
	// HasPassAttestation reports whether the lot has at least one PASS attestation.
	HasPassAttestation(ctx context.Context, lotID uuid.UUID) (bool, error)

	// CreateLotTx inserts a DRAFT lot AND marks the inbox key consumed in one
	// transaction. Returns ErrResourceExists when the inbox key was already seen
	// (duplicate event) so the use case can no-op, or when the object_id already
	// has a lot.
	CreateLotTx(ctx context.Context, lot entity.Lot, inboxKey string) error

	// RecordAttestationTx inserts an attestation, optionally flips the lot to
	// REJECTED (on FAIL of a DRAFT lot), and writes the outbox event, all in one tx.
	RecordAttestationTx(ctx context.Context, att entity.Attestation, rejectLot bool, outbox entity.OutboxEvent) error

	// CertifyTx flips a DRAFT lot to CERTIFIED and writes the outbox event in one
	// tx, conditional on the row still being DRAFT. Returns ErrResourceInvalid if
	// the conditional update affected no rows (already moved on / not DRAFT).
	CertifyTx(ctx context.Context, lotID uuid.UUID, outbox entity.OutboxEvent) (entity.Lot, error)

	// ScheduleTx flips a CERTIFIED lot to SCHEDULED for its week and writes the
	// outbox event in one tx, conditional on (a) the row still being CERTIFIED and
	// (b) fewer than cap lots already SCHEDULED for that week. Returns
	// ErrResourceInvalid when the conditional update affected no rows.
	ScheduleTx(ctx context.Context, lotID uuid.UUID, scheduledAt time.Time, weekCap int, outbox entity.OutboxEvent) (entity.Lot, error)
}
