// Package apperrors contains errors shared across the application's layers.
// The package name deliberately differs from the standard library's errors
// package so callers can use errors.Is and apperrors together without aliases.
package apperrors

import "errors"

var (
	ErrTitleRequired     = errors.New("title is required")
	ErrNothingToUpdate   = errors.New("nothing to update")
	ErrInvalidPagination = errors.New("invalid pagination")
)
