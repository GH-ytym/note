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

func (h *GroupHandler) ListMembers(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var uri GroupURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组 ID 不合法"})
		return
	}

	items, err := h.service.ListMembers(c.Request.Context(), uri.GroupID, userID)
	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请重新登录"})
		return
	case errors.Is(err, apperrors.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	case errors.Is(err, apperrors.ErrGroupAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询群成员失败"})
		return
	}

	response := make([]GroupMemberResponse, 0, len(items))
	for _, item := range items {
		if item.User == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "群成员资料缺失"})
			return
		}
		response = append(response, GroupMemberResponse{
			UserSummaryResponse: UserSummaryResponse{
				ID: item.User.ID, Username: item.User.Username, Suffix: item.User.Suffix,
				Nickname: item.User.Nickname, Avatar: item.User.Avatar,
			},
			JoinedAt: item.JoinedAt,
		})
	}
	c.JSON(http.StatusOK, response)
}

func (h *GroupHandler) DismissGroup(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var uri GroupURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组 ID 不合法"})
		return
	}

	err := h.service.DismissGroup(c.Request.Context(), uri.GroupID, userID)
	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请重新登录"})
	case errors.Is(err, apperrors.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, apperrors.ErrGroupAccessDenied), errors.Is(err, apperrors.ErrGroupDismissDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解散群组失败"})
	default:
		c.Status(http.StatusNoContent)
	}
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
		Policy:    item.Policy,
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
			Policy:    item.Policy,
			CreatedAt: item.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, result)
}

func (h *GroupHandler) GetInviteCode(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	var uri GroupURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组 ID 不合法"})
		return
	}

	code, err := h.service.GetInviteCode(c.Request.Context(), uri.GroupID, userID)
	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return

	case errors.Is(err, apperrors.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return

	case errors.Is(err, apperrors.ErrGroupAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{
			"error": "只有当前群成员可以获取邀请码",
		})
		return

	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取邀请码失败",
		})
		return
	}
	c.JSON(http.StatusOK, GroupInviteResponse{
		GroupID: uri.GroupID,
		Code:    code,
	})
}

func (h *GroupHandler) RefreshInviteCode(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var uri GroupURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组 ID 不合法"})
		return
	}

	code, err := h.service.RefreshInviteCode(
		c.Request.Context(),
		uri.GroupID,
		userID,
	)

	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return

	case errors.Is(err, apperrors.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return

	case errors.Is(err, apperrors.ErrGroupAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{
			"error": "只有仍在群内的群主可以刷新邀请码",
		})
		return

	case errors.Is(err, apperrors.ErrGroupInviteCodeConflict):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "暂时无法生成新邀请码，请稍后重试",
		})
		return

	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "刷新邀请码失败",
		})
		return
	}

	c.JSON(http.StatusOK, GroupInviteResponse{
		GroupID: uri.GroupID,
		Code:    code,
	})
}

func (h *GroupHandler) JoinGroup(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	var uri GroupURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组 ID 不合法"})
		return
	}

	var req JoinGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供邀请码"})
		return
	}

	err := h.service.JoinGroup(
		c.Request.Context(),
		uri.GroupID,
		userID,
		req.Code,
	)

	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请重新登录"})

	case errors.Is(err, apperrors.ErrGroupInviteCodeFormat):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

	case errors.Is(err, apperrors.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	case errors.Is(err, apperrors.ErrGroupInviteInvalid):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})

	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加入群组失败"})

	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *GroupHandler) QuitGroup(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	var uri GroupURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "群组 ID 不合法"})
		return
	}

	var req QuitGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求格式错误，target 必须是正整数",
		})
		return
	}

	err := h.service.QuitGroup(
		c.Request.Context(),
		uri.GroupID,
		userID,
		req.Target,
	)

	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请重新登录"})

	case errors.Is(err, apperrors.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	case errors.Is(err, apperrors.ErrGroupAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, apperrors.ErrGroupQuitConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})

	case errors.Is(err, apperrors.ErrGroupTransferTargetInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "退出群组失败"})

	default:
		c.Status(http.StatusNoContent)
	}
}
