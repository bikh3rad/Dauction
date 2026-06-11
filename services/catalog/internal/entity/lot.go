package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuctionMode picks the auction engine a lot will run on (CLAUDE.md §1).
// MONOSPACE_UPPERCASE protocol vocabulary; the value string IS the wire code.
type AuctionMode string

const (
	ModeDutch   AuctionMode = "DUTCH"   // live descending price (active)
	ModeVickrey AuctionMode = "VICKREY" // sealed second-price (timed, passive)
	ModeUniqBid AuctionMode = "UNIQBID" // lowest unique price (timed, passive)
)

// Valid reports whether m is a known auction mode.
func (m AuctionMode) Valid() bool {
	switch m {
	case ModeDutch, ModeVickrey, ModeUniqBid:
		return true
	default:
		return false
	}
}

// Timed reports whether the mode is a timed/passive auction (Vickrey/UniqBid),
// which carries an owner-set duration. DUTCH is live and has no duration.
func (m AuctionMode) Timed() bool {
	return m == ModeVickrey || m == ModeUniqBid
}

// LotState is the catalog-owned lifecycle of a lot up to scheduling (CLAUDE.md
// §7). The downstream auction services own the OPEN/HAMMER/.../COMPLETED states.
// Catalog drives: DRAFT -> CERTIFIED -> SCHEDULED, with REJECTED terminal.
type LotState string

const (
	LotDraft     LotState = "DRAFT"     // created from object.listed; awaiting certification
	LotCertified LotState = "CERTIFIED" // passed the inspector attestation gate
	LotScheduled LotState = "SCHEDULED" // admitted to a week's gallery (counts toward 32-cap)
	LotRejected  LotState = "REJECTED"  // terminal: failed certification / pulled
)

// Valid reports whether s is a known lot state.
func (s LotState) Valid() bool {
	switch s {
	case LotDraft, LotCertified, LotScheduled, LotRejected:
		return true
	default:
		return false
	}
}

// CanCertify reports whether a lot in this state may transition to CERTIFIED.
// Only a DRAFT lot may be certified.
func (s LotState) CanCertify() bool { return s == LotDraft }

// CanSchedule reports whether a lot in this state may transition to SCHEDULED.
// Only a CERTIFIED lot may be scheduled (certification gate, CLAUDE.md §2).
func (s LotState) CanSchedule() bool { return s == LotCertified }

// Lot is a catalog-owned gallery lot derived from a vault object.listed event.
// Money fields are int64 USDC cents (CLAUDE.md §0). Title/description are owner
// prose returned as-is; the client localizes nothing structural about them.
type Lot struct {
	ID                  uuid.UUID   `json:"id"`
	ObjectID            uuid.UUID   `json:"objectId"` // source vault object (unique)
	SellerAccountID     uuid.UUID   `json:"sellerAccountId"`
	Title               string      `json:"title"`
	Description         string      `json:"description"`
	Mode                AuctionMode `json:"atype"`        // DUTCH | VICKREY | UNIQBID
	DurationDays        *int32      `json:"durationDays"` // 2/5/7 for timed; nil for DUTCH
	ReserveCents        int64       `json:"reserveCents"`
	AppraisedValueCents int64       `json:"appraisedValueCents"`
	State               LotState    `json:"state"`
	ISOWeek             string      `json:"isoWeek"` // e.g. "2026-W23"
	CreatedAt           time.Time   `json:"createdAt"`
	ScheduledAt         *time.Time  `json:"scheduledAt"`

	// Inspector seal (§3.5). Populated once an Inspector seals the lot.
	CategoryCode   string     `json:"categoryCode"`
	Certified      bool       `json:"certified"`
	InspectorID    *uuid.UUID `json:"inspectorId,omitempty"`
	Authenticity   string     `json:"authenticity,omitempty"`   // GENUINE | COUNTERFEIT | INCONCLUSIVE
	ConditionGrade string     `json:"conditionGrade,omitempty"` // MINT | EXCELLENT | GOOD | FAIR | POOR
}

// InspectionVerdict is the Inspector's sealing decision (§3.5).
type InspectionVerdict string

const (
	VerdictApproved InspectionVerdict = "APPROVED"
	VerdictRejected InspectionVerdict = "REJECTED"
)

// Valid reports whether v is a known verdict.
func (v InspectionVerdict) Valid() bool { return v == VerdictApproved || v == VerdictRejected }

// ValidAuthenticity reports whether a is a known authenticity finding.
func ValidAuthenticity(a string) bool {
	switch a {
	case "GENUINE", "COUNTERFEIT", "INCONCLUSIVE":
		return true
	default:
		return false
	}
}

// ValidConditionGrade reports whether g is a known condition grade (empty allowed).
func ValidConditionGrade(g string) bool {
	switch g {
	case "", "MINT", "EXCELLENT", "GOOD", "FAIR", "POOR":
		return true
	default:
		return false
	}
}

// Inspection is an Inspector's sealing verdict on a lot — the certification gate
// record (§3.5). One per lot.
type Inspection struct {
	ID             uuid.UUID
	LotID          uuid.UUID
	InspectorID    uuid.UUID
	Verdict        InspectionVerdict
	Authenticity   string
	ConditionGrade string
	Notes          string
	SealedAt       time.Time
}
