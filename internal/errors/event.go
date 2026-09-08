package apperrors

import "errors"

var (
	ErrEventNotFound         = errors.New("event not found")
	ErrEventInvalidContent   = errors.New("invalid event content")
	ErrEventInvalidColor     = errors.New("invalid event color")
	ErrEventInvalidVersion   = errors.New("invalid event version")
	ErrInvalidEventTimeRange = errors.New("event end must be after start")
)
