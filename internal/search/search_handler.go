package search

import (
	"context"
	"errors"
	"net/http"
	apperrors "note/internal/errors"
	"note/internal/middleware"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct{ service Service }

func NewSearchHandler(service Service) *SearchHandler {
	return &SearchHandler{service: service}
}

func (h *SearchHandler) SearchTodos(c *gin.Context) {
	handleSearch(c, h.service.SearchTodos)
}

func (h *SearchHandler) SearchEvents(c *gin.Context) {
	handleSearch(c, h.service.SearchEvents)
}

func (h *SearchHandler) SearchAll(c *gin.Context) {
	handleSearch(c, h.service.SearchAll)
}

// 三个入口共用参数绑定与响应处理，聚合、排序和分页由 Repo 完成。
func handleSearch(c *gin.Context, run func(context.Context, ListQuery) (Result, error)) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var uri GroupSearchURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group"})
		return
	}
	var req SearchQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	page, err := run(c.Request.Context(), ListQuery{
		GroupID: uri.GroupID, UserID: userID,
		Keyword: req.Keyword, Page: req.Page, PageSize: req.PageSize,
	})
	if errors.Is(err, apperrors.ErrGroupUnauthenticated) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	if errors.Is(err, apperrors.ErrGroupAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权搜索该群组"})
		return
	}
	if errors.Is(err, apperrors.ErrInvalidSearchQuery) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"page": page,
	})
}
