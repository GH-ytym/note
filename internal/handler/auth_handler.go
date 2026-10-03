package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"note/internal/auth"
	apperrors "note/internal/errors"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service auth.Service
	tokens  *auth.TokenManager

	//接上redis client
	refresh *auth.RefreshStore
	//控制刷新 Cookie 是否只通过 HTTPS 发送
	cookieSecure bool
}

// Logout 撤销当前浏览器的刷新会话，成功后通知浏览器删除 Cookie。
func (h *AuthHandler) Logout(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	//先拿cookie
	raw, err := c.Cookie("note_refresh")
	if err != nil && !errors.Is(err, http.ErrNoCookie) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cookie 格式不正确"})
		return
	}
	if err == nil {
		//再删redis记录
		if err := h.refresh.Revoke(c.Request.Context(), raw); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "退出失败，请稍后重试"})
			return
		}
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("note_refresh", "", -1, "/", "", h.cookieSecure, true)
	c.Status(http.StatusNoContent)
	//jwt的清空在前端
}

func NewAuthHandler(
	service auth.Service,
	tokens *auth.TokenManager,
	refresh *auth.RefreshStore,
	cookieSecure bool,
) *AuthHandler {
	return &AuthHandler{
		service:      service,
		tokens:       tokens,
		refresh:      refresh,
		cookieSecure: cookieSecure,
	}
}

// 设置cookie
func (h *AuthHandler) setRefreshCookie(
	c *gin.Context,
	raw string,
) {
	c.SetSameSite(http.SameSiteStrictMode)

	c.SetCookie(
		"note_refresh",                   // Cookie 名字
		raw,                              // 原始刷新令牌
		int(auth.RefreshTTL/time.Second), // 有效期，单位为秒
		"/",                              // 网站所有路径都可携带
		"",                               // 不指定 Domain
		h.cookieSecure,                   // 是否只允许 HTTPS
		true,                             // HttpOnly
	)
}

// 这里包括第一次登录和重新登录，不论refreshtoken是否过期都会生成新的
func (h *AuthHandler) Login(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, err := h.service.Login(c.Request.Context(), req.Account, req.Password)
	if errors.Is(err, apperrors.ErrInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号或密码错误"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		return
	}

	//生成JWT token
	//后续访问受保护的接口时才会校验token的有效性
	tokenStr, err := h.tokens.Generate(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	ctx := c.Request.Context()
	// 原子替换 Redis 会话，成功后才覆盖浏览器 Cookie（一定是先写redis再写cookie）
	//这里忽略错误，因为第一次登录不会创建cookie，oldraw这个时候是""
	//cookie过期了这里也是""
	oldRaw, _ := c.Cookie("note_refresh")
	refreshToken, err := h.refresh.Replace(ctx, oldRaw, user.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "创建登录会话失败，请稍后重试",
		})
		return
	}

	// 通过 Cookie 把刷新令牌交给浏览器。
	h.setRefreshCookie(c, refreshToken)

	//要求客户端和中间缓存不缓存响应数据，确保安全性。
	c.Header("Cache-Control", "no-store")
	//返回用户数据和access token
	c.JSON(http.StatusOK, LoginResponse{
		ID:          user.ID,
		Name:        user.Username,
		Suffix:      user.Suffix,
		Account:     fmt.Sprintf("%s#%05d", user.Username, user.Suffix),
		AccessToken: tokenStr,
		TokenType:   "Bearer",
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "注册参数格式不正确"})
		return
	}
	user, err := h.service.Register(c.Request.Context(), req.Name, req.Email, req.Password)
	switch {
	case errors.Is(err, apperrors.ErrInvalidName), errors.Is(err, apperrors.ErrReservedName), errors.Is(err, apperrors.ErrInvalidPassword), errors.Is(err, apperrors.ErrInvalidEmail):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case errors.Is(err, apperrors.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case errors.Is(err, apperrors.ErrSuffixUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}
	c.JSON(http.StatusCreated, RegisterResponse{
		ID: user.ID, Name: user.Username, Suffix: user.Suffix,
		Account: fmt.Sprintf("%s#%05d", user.Username, user.Suffix),
	})
}

// 使用cookie里面的refresh token刷新jwt access token
func (h *AuthHandler) Refresh(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	//读取cookie里的refresh token（就是那个raw）
	raw, err := c.Cookie("note_refresh")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请重新登录",
		})
		return
	}

	ctx := c.Request.Context()

	// 查redis
	userID, err := h.refresh.Lookup(ctx, raw)
	if errors.Is(err, auth.ErrInvalidRefresh) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "登录已过期，请重新登录",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "登录服务暂时不可用",
		})
		return
	}

	//查数据库
	user, err := h.service.CurrUser(ctx, userID)
	if errors.Is(err, auth.ErrInvalidRefresh) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "账号不存在，请重新登录",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户失败",
		})
		return
	}

	//生成新jwt
	accessToken, err := h.tokens.Generate(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "生成访问令牌失败",
		})
		return
	}

	//轮换令牌
	newRaw, err := h.refresh.Rotate(ctx, raw, user.ID)
	if errors.Is(err, auth.ErrInvalidRefresh) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "刷新令牌已失效",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "刷新登录失败，请稍后重试",
		})
		return
	}

	// 通过响应头更新Cookie
	h.setRefreshCookie(c, newRaw)

	// 通过 JSON 把新的 access JWT 返回前端。
	c.JSON(http.StatusOK, LoginResponse{
		ID:          user.ID,
		Name:        user.Username,
		Suffix:      user.Suffix,
		Account:     fmt.Sprintf("%s#%05d", user.Username, user.Suffix),
		AccessToken: accessToken,
		TokenType:   "Bearer",
	})
}
