package middleware

import (
	"net/http"
	"strings"

	"note/internal/auth"

	"github.com/gin-gonic/gin"
)

// 统一使用这个 key，避免保存和读取时拼写不一致。
const UserIDKey = "user_id"
const AccessExpiresAtKey = "access_expires_at"

func RequireLogin(tm *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 读取请求头。
		//Authorization: Bearer abc.def.xyz
		header := c.GetHeader("Authorization")

		// 2. 按空白分成两部分
		//0:Bearer
		//1:tokenStr
		parts := strings.Fields(header)
		if len(parts) != 2 ||
			!strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "请先登录",
			})
			return
		}

		// 3. 验证 JWT 的签名、有效期等。
		claims, err := tm.Parse(parts[1])
		if err != nil {
			//要记得abort，不然return了还是会next
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "登录凭证无效或已过期",
			})
			return
		}

		// 4. 验证通过，把用户 ID 存进当前请求的 Context。
		c.Set(UserIDKey, claims.UserID)

		//SSE会用到
		//SSE请求保持很久，handler需要知道什么时候结束当前user的过期登录连接
		c.Set(AccessExpiresAtKey, claims.ExpiresAt.Time)

		// 5. 继续执行后面的 handler。
		c.Next()
	}
}
