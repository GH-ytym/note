package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireAuthRequest 防止外部网站通过普通表单触发 Cookie 认证操作。
// 配合当前同源 /api 代理使用，不应向任意外部 Origin 开放凭据 CORS。
func RequireAuthRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if c.GetHeader("X-Note-Request") != "1" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "缺少认证请求头"})
			return
		}
		c.Next()
	}
}
