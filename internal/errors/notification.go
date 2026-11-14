package apperrors

import "errors"

var ErrNotificationUnauthenticated = errors.New("请先登录")
var ErrCacheUnavailable = errors.New("notification cache unavailable")
