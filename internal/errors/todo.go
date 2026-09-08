package apperrors

import "errors"

var (
	ErrTodoNotFound          = errors.New("todo not found")
	ErrTodoTitleConflict     = errors.New("todo title already exists")
	ErrTodoConcurrentUpdate  = errors.New("todo was modified concurrently")
	ErrTodoInvalidContent    = errors.New("invalid todo content")
	ErrTodoInvalidColor      = errors.New("invalid todo color")
	ErrTodoInvalidVersion    = errors.New("invalid todo version")
	ErrInvalidRepeatMode     = errors.New("invalid repeat mode")
	ErrInvalidNotifyMode     = errors.New("invalid notify mode")
	ErrTodoStartsAtRequired  = errors.New("starts_at is required")
	ErrInvalidCalendarRange  = errors.New("invalid calendar range")
	ErrCustomDatesRequired   = errors.New("custom dates required")
	ErrCustomDatesNotAllowed = errors.New("custom dates not allowed")
)
