package biz

import "errors"

// Language-neutral biz sentinels (CLAUDE.md §0.7, §6). Handlers map these to HTTP
// status + machine codes via dto.HandleError; the value strings are internal only.
var (
	ErrResourceAccessDenied = errors.New("not authorized")
	ErrResourceNotFound     = errors.New("resource not found")
	ErrResourceExists       = errors.New("resource already exists")
	// ErrResourceInvalid covers unknown packages, malformed input, AND the
	// "out of credits" case (insufficient wallet balance on debit) — the client
	// maps the latter to OUT_OF_CREDITS (CLAUDE.md §5).
	ErrResourceInvalid = errors.New("invalid resource")
)
