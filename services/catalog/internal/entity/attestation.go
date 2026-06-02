package entity

import (
	"time"

	"github.com/google/uuid"
)

// AttestationResult is the inspector's seal on a lot (CLAUDE.md §1, §2). A PASS
// is the certification gate: a lot cannot be CERTIFIED/SCHEDULED without one.
type AttestationResult string

const (
	AttestPass AttestationResult = "PASS"
	AttestFail AttestationResult = "FAIL"
)

// Valid reports whether r is a known attestation result.
func (r AttestationResult) Valid() bool {
	switch r {
	case AttestPass, AttestFail:
		return true
	default:
		return false
	}
}

// Attestation is the inspector's recorded judgement on a lot. The append-only
// record backing the certification gate; a PASS unlocks certification.
type Attestation struct {
	ID          uuid.UUID         `json:"id"`
	LotID       uuid.UUID         `json:"lotId"`
	InspectorID uuid.UUID         `json:"inspectorId"`
	Result      AttestationResult `json:"result"`
	NotesRef    string            `json:"notesRef"`
	RecordedAt  time.Time         `json:"recordedAt"`
}
