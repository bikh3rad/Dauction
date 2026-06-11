package dto

import "application/internal/biz"

// RequestOTPReq is the body of POST /apis/auth/otp/request.
type RequestOTPReq struct {
	MobileE164 string `json:"mobileE164" validate:"required"`
	Purpose    string `json:"purpose" validate:"omitempty,oneof=SIGNUP LOGIN"`
}

// RequestOTPResp acknowledges an OTP issuance. DevCode is populated only in dev
// mode (no SMS provider); it is empty in production.
type RequestOTPResp struct {
	ExpiresInSecs int    `json:"expiresInSecs"`
	DevCode       string `json:"devCode,omitempty"`
}

// VerifyOTPReq is the body of POST /apis/auth/otp/verify.
type VerifyOTPReq struct {
	MobileE164 string `json:"mobileE164" validate:"required"`
	Code       string `json:"code" validate:"required"`
}

// SessionResp is returned by a successful sign-in/up. The token is the session
// bearer (dev: the account UUID; prod: a signed JWT).
type SessionResp struct {
	Token   string      `json:"token"`
	Created bool        `json:"created"`
	Account AccountResp `json:"account"`
}

// ToSessionResp maps an auth result to the API session response.
func ToSessionResp(res biz.AuthResult) SessionResp {
	return SessionResp{
		Token:   res.Token,
		Created: res.Created,
		Account: ToAccountResp(res.Account),
	}
}
