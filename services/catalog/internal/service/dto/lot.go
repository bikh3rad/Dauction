package dto

import (
	"application/internal/entity"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// isoTime formats a time as ISO-8601 UTC, the API's only time encoding.
func isoTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z07:00")
}

// LotResp is the language-neutral lot read model (enum codes, integer cents,
// ISO-8601 UTC). The client localizes; the API never pre-formats money or dates.
type LotResp struct {
	ID                  string `json:"id"`
	ObjectID            string `json:"objectId"`
	SellerAccountID     string `json:"sellerAccountId"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	Atype               string `json:"atype"`        // DUTCH | VICKREY | UNIQBID
	DurationDays        *int32 `json:"durationDays"` // null for DUTCH
	ReserveCents        int64  `json:"reserveCents"`
	AppraisedValueCents int64  `json:"appraisedValueCents"`
	State               string `json:"state"`     // DRAFT | CERTIFIED | SCHEDULED | REJECTED
	ISOWeek             string `json:"isoWeek"`   // e.g. "2026-W23"
	CreatedAt           string `json:"createdAt"` // ISO-8601 UTC
	ScheduledAt         string `json:"scheduledAt,omitempty"`
	// Inspector seal (§3.5) — surfaced on the gallery + auction detail.
	CategoryCode   string `json:"categoryCode,omitempty"`
	Certified      bool   `json:"certified"`
	InspectorID    string `json:"inspectorId,omitempty"`
	Authenticity   string `json:"authenticity,omitempty"`
	ConditionGrade string `json:"conditionGrade,omitempty"`
}

// AttestationResp is the inspector-seal read model on a lot.
type AttestationResp struct {
	ID          string `json:"id"`
	LotID       string `json:"lotId"`
	InspectorID string `json:"inspectorId"`
	Result      string `json:"result"` // PASS | FAIL
	NotesRef    string `json:"notesRef,omitempty"`
	RecordedAt  string `json:"recordedAt"`
}

// LotDetailResp is the lot detail returned by GET /apis/lots/{id}: the lot plus a
// summary of its attestations (the certification gate's evidence).
type LotDetailResp struct {
	Lot          LotResp           `json:"lot"`
	Certified    bool              `json:"certified"` // has at least one PASS attestation
	Attestations []AttestationResp `json:"attestations"`
}

// WeeklyResp is the public weekly gallery: this ISO week's SCHEDULED lots plus the
// supply cap so the client can render "n of 32".
type WeeklyResp struct {
	Week      string    `json:"week"`
	SupplyCap int       `json:"supplyCap"` // 32
	Lots      []LotResp `json:"lots"`
}

// AttestRequest is the body of POST /apis/admin/lots/{id}/attest.
type AttestRequest struct {
	InspectorID string `json:"inspectorId" validate:"required,uuid"`
	Result      string `json:"result"      validate:"required,oneof=PASS FAIL"`
	NotesRef    string `json:"notesRef"    validate:"omitempty,max=512"`
}

// ScheduleRequest is the body of POST /apis/admin/lots/{id}/schedule.
type ScheduleRequest struct {
	ScheduledAt string `json:"scheduledAt" validate:"omitempty"` // ISO-8601 UTC; defaults to now
}

// InspectRequest is the body of POST /apis/inspector/lots/{id}/inspect (§3.5).
// The inspector id is the gateway-injected caller, not part of the body.
type InspectRequest struct {
	Verdict        string `json:"verdict"        validate:"required,oneof=APPROVED REJECTED"`
	Authenticity   string `json:"authenticity"   validate:"required,oneof=GENUINE COUNTERFEIT INCONCLUSIVE"`
	ConditionGrade string `json:"conditionGrade" validate:"omitempty,oneof=MINT EXCELLENT GOOD FAIR POOR"`
	Notes          string `json:"notes"          validate:"omitempty,max=1024"`
}

// Validate mirrors the validate tags (no validator dependency in this module).
func (r InspectRequest) Validate() (entity.InspectionVerdict, error) {
	verdict := entity.InspectionVerdict(r.Verdict)
	if !verdict.Valid() {
		return "", fmt.Errorf("verdict must be APPROVED or REJECTED, got %q", r.Verdict)
	}
	if !entity.ValidAuthenticity(r.Authenticity) {
		return "", fmt.Errorf("authenticity must be GENUINE|COUNTERFEIT|INCONCLUSIVE, got %q", r.Authenticity)
	}
	if !entity.ValidConditionGrade(r.ConditionGrade) {
		return "", fmt.Errorf("invalid condition grade %q", r.ConditionGrade)
	}
	if len(r.Notes) > 1024 {
		return "", fmt.Errorf("notes too long")
	}

	return verdict, nil
}

// Validate checks the attest request against its validate tags (no validator
// dependency in this module; the tags document the contract and this mirrors
// them). Failure -> a wrapped error mapped to RESOURCE_INVALID by dto.HandleError.
func (r AttestRequest) Validate() (uuid.UUID, entity.AttestationResult, error) {
	inspectorID, err := uuid.Parse(r.InspectorID)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("inspectorId must be a UUID: %w", err)
	}

	result := entity.AttestationResult(r.Result)
	if !result.Valid() {
		return uuid.Nil, "", fmt.Errorf("result must be PASS or FAIL, got %q", r.Result)
	}

	if len(r.NotesRef) > 512 {
		return uuid.Nil, "", fmt.Errorf("notesRef too long")
	}

	return inspectorID, result, nil
}

// ParseScheduledAt parses the optional scheduledAt; an empty value yields the
// zero time (the use case defaults it to now).
func (r ScheduleRequest) ParseScheduledAt() (time.Time, error) {
	if r.ScheduledAt == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, r.ScheduledAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("scheduledAt must be ISO-8601 UTC: %w", err)
	}

	return t, nil
}

// ToLotResp maps an entity.Lot to its API response.
func ToLotResp(l entity.Lot) LotResp {
	resp := LotResp{
		ID:                  l.ID.String(),
		ObjectID:            l.ObjectID.String(),
		SellerAccountID:     l.SellerAccountID.String(),
		Title:               l.Title,
		Description:         l.Description,
		Atype:               string(l.Mode),
		DurationDays:        l.DurationDays,
		ReserveCents:        l.ReserveCents,
		AppraisedValueCents: l.AppraisedValueCents,
		State:               string(l.State),
		ISOWeek:             l.ISOWeek,
		CreatedAt:           isoTime(l.CreatedAt),
	}

	if l.ScheduledAt != nil {
		resp.ScheduledAt = isoTime(*l.ScheduledAt)
	}

	resp.CategoryCode = l.CategoryCode
	resp.Certified = l.Certified
	resp.Authenticity = l.Authenticity
	resp.ConditionGrade = l.ConditionGrade
	if l.InspectorID != nil {
		resp.InspectorID = l.InspectorID.String()
	}

	return resp
}

// ToLotResps maps a slice of lots.
func ToLotResps(lots []entity.Lot) []LotResp {
	out := make([]LotResp, 0, len(lots))
	for _, l := range lots {
		out = append(out, ToLotResp(l))
	}

	return out
}

// ToAttestationResp maps an entity.Attestation to its API response.
func ToAttestationResp(a entity.Attestation) AttestationResp {
	return AttestationResp{
		ID:          a.ID.String(),
		LotID:       a.LotID.String(),
		InspectorID: a.InspectorID.String(),
		Result:      string(a.Result),
		NotesRef:    a.NotesRef,
		RecordedAt:  isoTime(a.RecordedAt),
	}
}

// ToLotDetailResp builds the lot detail with its attestation summary.
func ToLotDetailResp(l entity.Lot, atts []entity.Attestation) LotDetailResp {
	out := LotDetailResp{
		Lot:          ToLotResp(l),
		Attestations: make([]AttestationResp, 0, len(atts)),
	}

	for _, a := range atts {
		if a.Result == entity.AttestPass {
			out.Certified = true
		}

		out.Attestations = append(out.Attestations, ToAttestationResp(a))
	}

	return out
}
