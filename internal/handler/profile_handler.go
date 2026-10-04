package handler

import (
	"errors"
	"net/http"
	"note/internal/middleware"
	"note/internal/model"
	"note/internal/profile"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProfileHandler struct{ service *profile.Service }

func NewProfileHandler(service *profile.Service) *ProfileHandler { return &ProfileHandler{service} }
func (h *ProfileHandler) response(c *gin.Context, user model.User, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请重新登录"})
	case errors.Is(err, profile.ErrInvalidImage):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, profile.ErrStorageUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存用户资料失败"})
	default:
		c.JSON(http.StatusOK, gin.H{
			"id": user.ID, "username": user.Username, "suffix": user.Suffix, "nickname": user.Nickname, "avatar": user.Avatar,
			"avatar_upload_enabled": h.service.UploadEnabled(),
		})
	}
}
func (h *ProfileHandler) Me(c *gin.Context) {
	user, err := h.service.Get(c.Request.Context(), c.GetUint(middleware.UserIDKey))
	h.response(c, user, err)
}
func (h *ProfileHandler) UploadAvatar(c *gin.Context) {
	// ParseMultipartForm 可能把大文件落盘，因此这里以流式方式读取，不调用 FormFile。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, profile.MaxUploadBytes+(64<<10))
	reader, err := c.Request.MultipartReader()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请用 multipart/form-data 上传 avatar 文件"})
		return
	}
	for {
		part, err := reader.NextPart()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 avatar 文件或文件过大"})
			return
		}
		if part.FormName() != "avatar" || part.FileName() == "" {
			part.Close()
			continue
		}
		user, err := h.service.Upload(c.Request.Context(), c.GetUint(middleware.UserIDKey), part)
		part.Close()
		h.response(c, user, err)
		return
	}
}
func (h *ProfileHandler) RemoveAvatar(c *gin.Context) {
	user, err := h.service.Remove(c.Request.Context(), c.GetUint(middleware.UserIDKey))
	h.response(c, user, err)
}
