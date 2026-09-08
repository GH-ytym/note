package handler

import (
	"errors"
	"net/http"
	apperrors "note/internal/errors"
	todoapp "note/internal/todo"
	"time"

	"github.com/gin-gonic/gin"
)

// CreateTodo creates a Todo and sends the error via gin
func (h *TodoHandler) CreateTodo(c *gin.Context) {
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
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// ListTodos returns a list of Todos
func (h *TodoHandler) ListTodos(c *gin.Context) {
	var q ListTodosQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
		return
	}

	page, err := h.service.List(c.Request.Context(), todoapp.ListQuery{
		Page:     q.Page,
		PageSize: q.PageSize,
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
	id, ok := parseTodoID(c)
	if !ok {
		return
	}

	item, err := h.service.Get(c.Request.Context(), id)

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

	c.JSON(http.StatusOK, item)
}

// PatchTodo updates a Todo with optimistic locking
func (h *TodoHandler) PatchTodo(c *gin.Context) {
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
		AllDone:    req.AllDone,
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
	item, err := h.service.Patch(c.Request.Context(), id, command)

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
	id, ok := parseTodoID(c)
	if !ok {
		return
	}

	err := h.service.Delete(c.Request.Context(), id)
	if errors.Is(err, apperrors.ErrTodoNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "todo not found",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete todo",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TodoHandler) PatchOccurrenceDone(c *gin.Context) {
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
		occursOn,
		*req.Done,
	)

	if errors.Is(err, apperrors.ErrTodoNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "todo not found",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update occurrence",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"todo_id":         id,
		"occurs_on":       occursOn.Format(time.DateOnly),
		"occurrence_done": *req.Done,
	})
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
