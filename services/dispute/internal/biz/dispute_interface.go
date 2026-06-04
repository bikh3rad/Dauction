package biz

import (
	"application/internal/entity"
	"context"

	"github.com/google/uuid"
)

// OpenParams is the input to opening a dispute (handler -> use case).
type OpenParams struct {
	TradeID     string
	Claimant    uuid.UUID // the buyer raising the claim
	Respondent  uuid.UUID // the seller (derived/supplied by the gateway/escrow read)
	ReasonCode  entity.ReasonCode
	EvidenceRef string
}

// DisputeView bundles a dispute with its immutable audit trail for the read API.
type DisputeView struct {
	Dispute entity.Dispute
	Events  []entity.DisputeEvent
}

// ListFilter narrows the admin queue. Empty State means "all".
type ListFilter struct {
	State entity.State
}

// UsecaseDispute is the dispute-court use case consumed by handlers. It owns the
// OPEN -> UNDER_REVIEW -> RESOLVED (+ WITHDRAWN) state machine and the immutable
// audit trail; every mutation appends a dispute_event row.
type UsecaseDispute interface {
	// Open creates an OPEN dispute for a trade and emits dispute.opened. Only one
	// non-terminal dispute may exist per trade (else ErrResourceExists).
	Open(ctx context.Context, p OpenParams) (entity.Dispute, error)
	// Get returns the dispute for a trade plus its audit trail. Caller must be a
	// party (claimant/respondent) or admin, else ErrResourceAccessDenied.
	Get(ctx context.Context, tradeID string, caller uuid.UUID, admin bool) (DisputeView, error)
	// AddEvidence appends an EVIDENCE_ADDED audit row. Either party only; the
	// dispute must still be non-terminal.
	AddEvidence(ctx context.Context, tradeID string, caller uuid.UUID, detailRef string) error
	// StartReview moves a dispute OPEN -> UNDER_REVIEW (admin). Illegal from any
	// other state -> ErrResourceInvalid.
	StartReview(ctx context.Context, disputeID uuid.UUID, admin uuid.UUID) (entity.Dispute, error)
	// Resolve sets the ruling and moves UNDER_REVIEW -> RESOLVED (admin), emitting
	// dispute.resolved. Ruling is immutable once set; re-resolve -> ErrResourceInvalid.
	Resolve(ctx context.Context, tradeID string, ruling entity.Ruling, ruledBy uuid.UUID) (entity.Dispute, error)
	// Withdraw moves a dispute OPEN -> WITHDRAWN. Claimant only, OPEN state only.
	Withdraw(ctx context.Context, tradeID string, caller uuid.UUID) (entity.Dispute, error)
	// ListByState returns the admin queue, optionally filtered by state.
	ListByState(ctx context.Context, f ListFilter) ([]entity.Dispute, error)
}

// RepositoryDispute is the persistence seam (implemented by internal/repo, mocked
// in tests). State transitions that emit an event do so atomically with the
// outbox row AND the appended audit row via the *Tx methods.
type RepositoryDispute interface {
	// CreateTx inserts a new OPEN dispute, its OPENED audit row, and the
	// dispute.opened outbox row in one transaction. Returns ErrResourceExists if a
	// non-terminal dispute already exists for the trade.
	CreateTx(ctx context.Context, d entity.Dispute, audit entity.DisputeEvent, outbox entity.OutboxEvent) (entity.Dispute, error)
	// GetByTrade returns the latest dispute for a trade or ErrResourceNotFound.
	GetByTrade(ctx context.Context, tradeID string) (entity.Dispute, error)
	// GetByID returns a dispute by id or ErrResourceNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (entity.Dispute, error)
	// ListEvents returns the immutable audit trail for a dispute, oldest first.
	ListEvents(ctx context.Context, disputeID uuid.UUID) ([]entity.DisputeEvent, error)
	// AppendEvent inserts an immutable audit row (no state change).
	AppendEvent(ctx context.Context, audit entity.DisputeEvent) error
	// TransitionTx writes the new state (CAS on from-state) and appends the audit
	// row in one transaction. Returns ErrResourceInvalid if the row is not in the
	// expected from-state. Used for OPEN -> UNDER_REVIEW and OPEN -> WITHDRAWN.
	TransitionTx(ctx context.Context, id uuid.UUID, from, to entity.State, audit entity.DisputeEvent) (entity.Dispute, error)
	// ResolveTx sets ruling+RESOLVED (CAS on from-state), appends the RULED audit
	// row, and writes the dispute.resolved outbox row in one transaction. Returns
	// ErrResourceInvalid if the row is not in the expected from-state.
	ResolveTx(ctx context.Context, id uuid.UUID, from entity.State, ruling entity.Ruling, ruledBy uuid.UUID, audit entity.DisputeEvent, outbox entity.OutboxEvent) (entity.Dispute, error)
	// ListByState returns disputes, optionally filtered by state (empty = all),
	// newest first.
	ListByState(ctx context.Context, state entity.State) ([]entity.Dispute, error)
}
