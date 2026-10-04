package apperrors

import "errors"

var (
	ErrEventNotFound         = errors.New("event not found")
	ErrEventConcurrentUpdate = errors.New("event was modified concurrently")
	ErrEventInvalidContent   = errors.New("invalid event content")
	ErrEventInvalidColor     = errors.New("invalid event color")
	ErrEventInvalidVersion   = errors.New("invalid event version")
	ErrInvalidEventTimeRange = errors.New("event end must be after start")
	ErrEventInvalidGroup     = errors.New("invalid event group")
	ErrEventEditDenied       = errors.New("event edit permission denied")
	ErrEventDeleteDenied     = errors.New("only the event creator can delete it")
	ErrEventPermissionDenied = errors.New("only the event creator can manage permissions")
	ErrEventRoleInvalid      = errors.New("invalid event member role")
	ErrEventMemberNotFound   = errors.New("event member not found in this group")
)
