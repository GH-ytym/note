package todo

import "errors"

var (
	ErrNotFound              = errors.New("todo not found")
	ErrTitleConflict         = errors.New("todo title already exists")
	ErrConcurrentUpdate      = errors.New("todo was modified concurrently")
	ErrInvalidContent        = errors.New("invalid todo content")
	ErrInvalidColor          = errors.New("invalid todo color")
	ErrNothingToUpdate       = errors.New("nothing to update")
	ErrInvalidVersion        = errors.New("invalid todo version")
	ErrInvalidPagination     = errors.New("invalid pagination")
	ErrInvalidRepeatMode     = errors.New("invalid repeat mode")
	ErrInvalidNotifyMode     = errors.New("invalid notify mode")
	ErrStartsAtRequired      = errors.New("starts_at is required")
	ErrInvalidCalendarRange  = errors.New("invalid calendar range")
	ErrCustomDatesRequired   = errors.New("custom dates required")
	ErrCustomDatesNotAllowed = errors.New("custom dates not allowed")
	ErrTitleRequired         = errors.New("title is required")
)
