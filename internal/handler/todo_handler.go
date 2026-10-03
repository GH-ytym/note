package handler

import (
	"errors"
	"net/http"
	apperrors "note/internal/errors"
	"note/internal/middleware"
	todoapp "note/internal/todo"
	"time"

	"github.com/gin-gonic/gin"
)

// CreateTodo creates a Todo and sends the error via gin
func (h *TodoHandler) CreateTodo(c *gin.Context) {
	//先验证userID
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}
	var req CreateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	customDates, err := parseCustomDates(req.CustomDates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid custom date",
		})
		return
	}

	// 组装command
	command := todoapp.CreateCommand{
		Title:       req.Title,
		Content:     req.Content,
		Color:       req.Color,
		StartsAt:    req.StartsAt,
		RepeatMode:  req.RepeatMode,
		NotifyMode:  req.NotifyMode,
		CustomDates: customDates,

		GroupID:   req.GroupID,
		CreatorID: userID,
	}
	//进入service，传递command
	item, err := h.service.Create(
		c.Request.Context(),
		command,
	)
	if errors.Is(err, apperrors.ErrTitleRequired) ||
		errors.Is(err, apperrors.ErrTodoInvalidContent) ||
		errors.Is(err, apperrors.ErrTodoInvalidColor) ||
		errors.Is(err, apperrors.ErrInvalidRepeatMode) ||
		errors.Is(err, apperrors.ErrInvalidNotifyMode) ||
		errors.Is(err, apperrors.ErrTodoStartsAtRequired) ||
		errors.Is(err, apperrors.ErrCustomDatesRequired) ||
		errors.Is(err, apperrors.ErrCustomDatesNotAllowed) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, apperrors.ErrTodoTitleConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "title already exists"})
		return
	}
	if errors.Is(err, apperrors.ErrGroupUnauthenticated) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}
	if errors.Is(err, apperrors.ErrGroupAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "无权在该群组创建 Todo",
		})
		return
	}
	if errors.Is(err, apperrors.ErrTodoInvalidGroup) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// ListTodos returns a list of Todos
func (h *TodoHandler) ListTodos(c *gin.Context) {
	//检查user
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}

	//拿到group
	var uri GroupTodosURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var q ListTodosQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
		return
	}

	page, err := h.service.List(c.Request.Context(), todoapp.ListQuery{
		Page:     q.Page,
		PageSize: q.PageSize,
		UserID:   userID,
		GroupID:  uri.GroupID,
	})
	if errors.Is(err, apperrors.ErrInvalidPagination) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list todos",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":      page.Items,
		"page":      page.Page,
		"page_size": page.PageSize,
		"total":     page.Total,
	})
}

// GetTodo returns a Todo by ID.
func (h *TodoHandler) GetTodo(c *gin.Context) {
	// UserIDKey 中的值由 RequireLogin 验证 JWT 后写入，不从请求参数读取。
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	id, ok := parseTodoID(c)
	if !ok {
		return
	}

	item, err := h.service.Get(c.Request.Context(), id, userID)

	if errors.Is(err, apperrors.ErrGroupUnauthenticated) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	if errors.Is(err, apperrors.ErrGroupAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该 Todo 所属群组"})
		return
	}

	if errors.Is(err, apperrors.ErrTodoNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "todo not found",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get todo",
		})
		return
	}

	c.JSON(http.StatusOK, newTodoDetailResponse(item))
}

// PatchTodo updates a Todo with optimistic locking
func (h *TodoHandler) PatchTodo(c *gin.Context) {
	//依旧先登录
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	//调整id
	id, ok := parseTodoID(c)
	if !ok {
		return
	}

	var req PatchTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	command := todoapp.PatchCommand{
		Title:      req.Title,
		Content:    req.Content,
		Color:      req.Color,
		StartsAt:   req.StartsAt,
		RepeatMode: req.RepeatMode,
		NotifyMode: req.NotifyMode,
		Version:    req.Version,
	}

	if req.CustomDates != nil {
		customDates, err := parseCustomDates(*req.CustomDates)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid custom date"})
			return
		}
		command.CustomDates = &customDates
	}
	item, err := h.service.Patch(
		c.Request.Context(),
		id,
		userID,
		command,
	)

	if errors.Is(err, apperrors.ErrTitleRequired) ||
		errors.Is(err, apperrors.ErrTodoInvalidContent) ||
		errors.Is(err, apperrors.ErrTodoInvalidColor) ||
		errors.Is(err, apperrors.ErrNothingToUpdate) ||
		errors.Is(err, apperrors.ErrTodoInvalidVersion) ||
		errors.Is(err, apperrors.ErrInvalidRepeatMode) ||
		errors.Is(err, apperrors.ErrInvalidNotifyMode) ||
		errors.Is(err, apperrors.ErrTodoStartsAtRequired) ||
		errors.Is(err, apperrors.ErrCustomDatesRequired) ||
		errors.Is(err, apperrors.ErrCustomDatesNotAllowed) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	if errors.Is(err, apperrors.ErrTodoTitleConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "title already exists"})
		return
	}
	if errors.Is(err, apperrors.ErrTodoNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})
		return
	}
	if errors.Is(err, apperrors.ErrTodoConcurrentUpdate) {
		c.JSON(http.StatusConflict, gin.H{"error": "version conflict"})
		return
	}
	if errors.Is(err, apperrors.ErrGroupUnauthenticated) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	if errors.Is(err, apperrors.ErrGroupAccessDenied) ||
		errors.Is(err, apperrors.ErrTodoEditDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update todo",
		})
		return
	}

	c.JSON(http.StatusOK, item)
}

// DeleteTodo deletes a Todo by ID.
func (h *TodoHandler) DeleteTodo(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	id, ok := parseTodoID(c)
	if !ok {
		return
	}

	err := h.service.Delete(c.Request.Context(), id, userID)

	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})

	case errors.Is(err, apperrors.ErrTodoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})

	case errors.Is(err, apperrors.ErrGroupAccessDenied),
		errors.Is(err, apperrors.ErrTodoDeleteDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})

	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete todo",
		})

	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *TodoHandler) PatchOccurrenceDone(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	id, ok := parseTodoID(c)
	if !ok {
		return
	}

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to load timezone",
		})
		return
	}

	occursOn, err := time.ParseInLocation(
		time.DateOnly,
		c.Param("date"),
		location,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid occurrence date",
		})
		return
	}

	var req PatchOccurrenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "done is required",
		})
		return
	}

	err = h.service.SetOccurrenceDone(
		c.Request.Context(),
		id,
		userID,
		occursOn,
		*req.Done,
	)

	if errors.Is(err, apperrors.ErrGroupUnauthenticated) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	if errors.Is(err, apperrors.ErrGroupAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, apperrors.ErrTodoNotFound) || errors.Is(err, apperrors.ErrTodoOccurrenceNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update occurrence",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TodoHandler) GetOccurrenceCompletions(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	todoID, ok := parseTodoID(c)
	if !ok {
		return
	}
	occursOn, ok := parseOccurrenceDate(c)
	if !ok {
		return
	}
	items, err := h.service.GetOccurrenceCompletions(c.Request.Context(), todoID, userID, occursOn)
	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	case errors.Is(err, apperrors.ErrGroupAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	case errors.Is(err, apperrors.ErrTodoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list completions"})
		return
	}
	response := OccurrenceCompletionsResponse{
		TodoID: todoID,
		Users:  make([]CompletedUserResponse, 0, len(items)),
	}
	for _, item := range items {
		response.Users = append(response.Users, CompletedUserResponse{
			UserSummaryResponse: UserSummaryResponse{
				ID: item.User.ID, Username: item.User.Username, Suffix: item.User.Suffix,
				Nickname: item.User.Nickname, Avatar: item.User.Avatar,
			},
			CompletedAt: item.CompletedAt,
		})
	}
	response.CompletedCount = len(response.Users)
	c.JSON(http.StatusOK, response)
}

// 解析从前端返回的string时间数组
func parseCustomDates(values []string) ([]time.Time, error) {
	dates := make([]time.Time, 0, len(values))

	for _, value := range values {
		date, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return nil, err
		}

		dates = append(dates, date)
	}

	return dates, nil
}
