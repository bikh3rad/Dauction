package biz

import "errors"

var (
	ErrResourceAccessDenied = errors.New("not authorized")
	ErrResourceNotFound     = errors.New("resource not found")
	ErrResourceExists       = errors.New("resource already exists")
	ErrResourceInvalid      = errors.New("invalid resource")
)
