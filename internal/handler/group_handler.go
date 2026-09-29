package handler

import (
	"errors"
	"net/http"

	apperrors "note/internal/errors"
	"note/internal/group"
	"note/internal/middleware"

	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	service group.Service
}

func NewGroupHandler(service group.Service) *GroupHandler {
	return &GroupHandler{service: service}
}

func (h *GroupHandler) CreateGroup(c *gin.Context) {
	// 身份来自认证中间件，不能由请求体指定。

	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}

	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "群名必填，且不能超过80个字符",
		})
		return
	}

	item, err := h.service.Create(
		c.Request.Context(),
		userID,
		req.Name,
	)

	switch {
	case errors.Is(err, apperrors.ErrGroupNameInvalid):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return

	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户身份无效，请重新登录",
		})
		return

	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建群组失败",
		})
		return
	}

	c.JSON(http.StatusCreated, GroupResponse{
		ID:        item.ID,
		Name:      item.Name,
		OwnerID:   item.OwnerID,
		CreatedAt: item.CreatedAt,
	})
}

func (h *GroupHandler) MyGroups(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}
	items, err := h.service.MyGroups(c.Request.Context(), userID)
	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return

	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询群组失败",
		})
		return
	}

	result := make([]GroupResponse, 0, len(items))
	for _, item := range items {
		result = append(result, GroupResponse{
			ID:        item.ID,
			Name:      item.Name,
			OwnerID:   item.OwnerID,
			CreatedAt: item.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, result)
}
