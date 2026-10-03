package apperrors

import "errors"

var (
	ErrGroupNameInvalid = errors.New("群名不能为空，且不能超过80个字符")

	ErrGroupUnauthenticated    = errors.New("用户身份无效")
	ErrGroupAccessDenied       = errors.New("无权访问该群组")
	ErrGroupNotFound           = errors.New("群组不存在")
	ErrGroupInviteCodeConflict = errors.New("新邀请码与当前邀请码相同")
	ErrGroupInviteCodeFormat   = errors.New("邀请码必须为6位数字或大写字母")
	ErrGroupInviteInvalid      = errors.New("邀请码无效或已刷新")
)
