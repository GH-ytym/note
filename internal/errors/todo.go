package apperrors

import "errors"

var (
	ErrTodoNotFound           = errors.New("todo not found")
	ErrTodoTitleConflict      = errors.New("todo title already exists")
	ErrTodoConcurrentUpdate   = errors.New("todo was modified concurrently")
	ErrTodoInvalidContent     = errors.New("invalid todo content")
	ErrTodoInvalidColor       = errors.New("invalid todo color")
	ErrTodoInvalidVersion     = errors.New("invalid todo version")
	ErrInvalidRepeatMode      = errors.New("invalid repeat mode")
	ErrInvalidNotifyMode      = errors.New("invalid notify mode")
	ErrTodoStartsAtRequired   = errors.New("starts_at is required")
	ErrInvalidCalendarRange   = errors.New("invalid calendar range")
	ErrCustomDatesRequired    = errors.New("custom dates required")
	ErrCustomDatesNotAllowed  = errors.New("custom dates not allowed")
	ErrTodoInvalidGroup       = errors.New("群组 ID 不合法")
	ErrTodoEditDenied         = errors.New("没有该 Todo 的编辑权限")
	ErrTodoDeleteDenied       = errors.New("只有创建者可以删除该 Todo")
	ErrTodoOccurrenceNotFound = errors.New("这天没有发生该 Todo")
	ErrTodoRoleInvalid        = errors.New("权限不合法，创建者必须保持 editor")
	ErrTodoInvalidMembers     = errors.New("成员列表必须包含 1～100 个有效用户 ID")
	ErrTodoPermissionDenied   = errors.New("只有创建者可以调整 Todo 成员权限")
	ErrTodoMemberNotFound     = errors.New("部分目标用户不是当前群成员，或缺少 Todo 授权记录")
)
