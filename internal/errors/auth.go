package apperrors

import "errors"

var (
	ErrInvalidCredentials = errors.New("账号或密码错误")
	ErrInvalidName        = errors.New("注册名必须为 2～20 位汉字、英文字母、数字或下划线")
	ErrReservedName       = errors.New("不能使用这个注册名")
	ErrInvalidPassword    = errors.New("密码必须为 8～20 位英文字母、数字或下划线")
	ErrInvalidEmail       = errors.New("邮箱格式不正确")
	ErrEmailTaken         = errors.New("邮箱已被注册")
	ErrAccountTaken       = errors.New("完整账号已被使用")
	ErrSuffixUnavailable  = errors.New("暂时无法分配账号，请重试")
	ErrInvalidRefresh     = errors.New("刷新令牌无效或已过期")
)
