package handler

import (
	"errors"
	"net/http"
	todoapp "note/internal/todo"
	"time"

	"github.com/gin-gonic/gin"
)

// CreateTodo creates a Todo and sends the error via gin
func (h *Handler) CreateTodo(c *gin.Context) {
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
		Content:     req.Content,
		Color:       req.Color,
		StartsAt:    req.StartsAt,
		RepeatMode:  req.RepeatMode,
		NotifyMode:  req.NotifyMode,
		CustomDates: customDates,
	}
	//进入service，传递command
	item, err := h.todoService.Create(
		c.Request.Context(),
		command,
	)
	if errors.Is(err, todoapp.ErrInvalidContent) ||
		errors.Is(err, todoapp.ErrInvalidColor) ||
		errors.Is(err, todoapp.ErrInvalidRepeatMode) ||
		errors.Is(err, todoapp.ErrInvalidNotifyMode) ||
		errors.Is(err, todoapp.ErrStartsAtRequired) ||
		errors.Is(err, todoapp.ErrCustomDatesRequired) ||
		errors.Is(err, todoapp.ErrCustomDatesNotAllowed) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, todoapp.ErrContentConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "content already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// ListTodos returns a list of Todos
func (h *Handler) ListTodos(c *gin.Context) {
	var q ListTodosQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
		return
	}

	page, err := h.todoService.List(c.Request.Context(), todoapp.ListQuery{
		Page:     q.Page,
		PageSize: q.PageSize,
	})
	if errors.Is(err, todoapp.ErrInvalidPagination) {
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
func (h *Handler) GetTodo(c *gin.Context) {
	id, ok := parseTodoID(c)
	if !ok {
		return
	}

	item, err := h.todoService.Get(c.Request.Context(), id)

	if errors.Is(err, todoapp.ErrNotFound) {
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
func (h *Handler) PatchTodo(c *gin.Context) {
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
	item, err := h.todoService.Patch(c.Request.Context(), id, command)

	if errors.Is(err, todoapp.ErrInvalidContent) ||
		errors.Is(err, todoapp.ErrInvalidColor) ||
		errors.Is(err, todoapp.ErrNothingToUpdate) ||
		errors.Is(err, todoapp.ErrInvalidVersion) ||
		errors.Is(err, todoapp.ErrInvalidRepeatMode) ||
		errors.Is(err, todoapp.ErrInvalidNotifyMode) ||
		errors.Is(err, todoapp.ErrStartsAtRequired) ||
		errors.Is(err, todoapp.ErrCustomDatesRequired) ||
		errors.Is(err, todoapp.ErrCustomDatesNotAllowed) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	if errors.Is(err, todoapp.ErrContentConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": "content already exists"})
		return
	}
	if errors.Is(err, todoapp.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "todo not found"})
		return
	}
	if errors.Is(err, todoapp.ErrConcurrentUpdate) {
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
func (h *Handler) DeleteTodo(c *gin.Context) {
	id, ok := parseTodoID(c)
	if !ok {
		return
	}

	err := h.todoService.Delete(c.Request.Context(), id)
	if errors.Is(err, todoapp.ErrNotFound) {
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

func (h *Handler) PatchOccurrenceDone(c *gin.Context) {
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

	err = h.todoService.SetOccurrenceDone(
		c.Request.Context(),
		id,
		occursOn,
		*req.Done,
	)

	if errors.Is(err, todoapp.ErrNotFound) {
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
