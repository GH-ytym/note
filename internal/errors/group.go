package apperrors

import "errors"

var (
	ErrGroupNameInvalid = errors.New(
		"群名不能为空，且不能超过80个字符",
	)

	ErrGroupUnauthenticated = errors.New(
		"用户身份无效",
	)
)
