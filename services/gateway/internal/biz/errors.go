package biz

import "errors"

// Sentinel errors. These map to language-neutral HTTP codes in dto.HandleError.
// The gateway owns no domain DB; these cover edge concerns (auth, tier/KYC guard,
// rate limiting and upstream proxy failures).
var (
	// ErrResourceAccessDenied — caller is unauthenticated / token invalid (401).
	ErrResourceAccessDenied = errors.New("not authorized")
	// ErrResourceNotFound — no route matches (404) or upstream returns not-found.
	ErrResourceNotFound = errors.New("resource not found")
	// ErrResourceExists — reserved for symmetry with the template (409).
	ErrResourceExists = errors.New("resource already exists")
	// ErrResourceInvalid — malformed request the gateway can reject early (400).
	ErrResourceInvalid = errors.New("invalid resource")

	// ErrTierRequired — route needs MEMBER/VIP but caller is GUEST (403).
	ErrTierRequired = errors.New("TIER_REQUIRED")
	// ErrKycRequired — route needs KYC APPROVED but caller is not (403).
	ErrKycRequired = errors.New("KYC_REQUIRED")
	// ErrRateLimited — client exceeded the configured rate window (429).
	ErrRateLimited = errors.New("RATE_LIMITED")
	// ErrUpstreamUnavailable — the resolved upstream service could not be reached (502).
	ErrUpstreamUnavailable = errors.New("UPSTREAM_UNAVAILABLE")
)
