package apperrors

import "errors"

var (
	ErrInvalidSearchQuery = errors.New("invalid search query")
	ErrNotImplemented     = errors.New("not implemented")
)
