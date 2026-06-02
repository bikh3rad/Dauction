package dto

import "application/internal/entity"

// StartKycReq begins a KYC flow.
type StartKycReq struct {
	DocType string `json:"docType" validate:"required,oneof=EMIRATES_ID PASSPORT"`
	DocRef  string `json:"docRef"  validate:"required,min=1,max=255"`
	Phone   string `json:"phone"   validate:"required,e164"`
}

// StartKycResp confirms a challenge was issued. devCode is only present in
// non-production environments to ease local testing.
type StartKycResp struct {
	SubmissionID string `json:"submissionId"`
	ChallengeID  string `json:"challengeId"`
	State        string `json:"state"`
	ExpiresAt    string `json:"expiresAt"`
	DevCode      string `json:"devCode,omitempty"`
}

// VerifyKycReq submits an OTP code.
type VerifyKycReq struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

// RejectKycReq carries an admin rejection reason (machine code).
type RejectKycReq struct {
	Reason string `json:"reason" validate:"required,max=64"`
}

// SubmissionResp is the language-neutral view of a submission.
type SubmissionResp struct {
	ID              string `json:"id"`
	AccountID       string `json:"accountId"`
	DocType         string `json:"docType"`
	DocRef          string `json:"docRef"`
	Phone           string `json:"phone"`
	State           string `json:"state"`
	RejectionReason string `json:"rejectionReason,omitempty"`
	SubmittedAt     string `json:"submittedAt"`
	DecidedAt       string `json:"decidedAt,omitempty"`
}

// SubmissionListResp wraps a queue of submissions.
type SubmissionListResp struct {
	Count       int               `json:"count"`
	Submissions []*SubmissionResp `json:"submissions"`
}

// ToSubmissionResp maps an entity to its response DTO.
func ToSubmissionResp(s *entity.Submission) *SubmissionResp {
	if s == nil {
		return nil
	}

	resp := &SubmissionResp{
		ID:              s.ID.String(),
		AccountID:       s.AccountID.String(),
		DocType:         string(s.DocType),
		DocRef:          s.DocRef,
		Phone:           s.Phone,
		State:           string(s.State),
		RejectionReason: s.RejectionReason,
		SubmittedAt:     s.SubmittedAt.UTC().Format(isoUTC),
	}

	if s.DecidedAt != nil {
		resp.DecidedAt = s.DecidedAt.UTC().Format(isoUTC)
	}

	return resp
}

// ToSubmissionResps maps a slice of entities to a list response.
func ToSubmissionResps(es []entity.Submission) *SubmissionListResp {
	resps := make([]*SubmissionResp, 0, len(es))
	for i := range es {
		resps = append(resps, ToSubmissionResp(&es[i]))
	}

	return &SubmissionListResp{
		Count:       len(resps),
		Submissions: resps,
	}
}

const isoUTC = "2006-01-02T15:04:05Z07:00"
