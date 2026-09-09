package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	apperrors "note/internal/errors"
	"note/internal/event"

	"github.com/gin-gonic/gin"
)

// EventHandler translates Event HTTP requests into Event service calls.
type EventHandler struct {
	service event.EventService
}

func (h *EventHandler) GetEvent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "invalid event id"})
		return
	}
	item, err := h.service.Get(c.Request.Context(), uint(id))
	if eventResponseError(c, err) {
		return
	}
	c.JSON(200, item)
}

func (h *EventHandler) PatchEvent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "invalid event id"})
		return
	}
	var req struct {
		Title    *string    `json:"title" binding:"omitempty,max=50"`
		Content  *string    `json:"content" binding:"omitempty,max=500"`
		StartsAt *time.Time `json:"starts_at"`
		EndsAt   *time.Time `json:"ends_at"`
		Version  uint       `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	item, err := h.service.Patch(c.Request.Context(), uint(id), event.PatchCommand{Title: req.Title, Content: req.Content, StartsAt: req.StartsAt, EndsAt: req.EndsAt, Version: req.Version})
	if eventResponseError(c, err) {
		return
	}
	c.JSON(200, item)
}

func eventResponseError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	status := http.StatusInternalServerError
	message := "failed to read or update event"
	switch {
	case errors.Is(err, apperrors.ErrEventNotFound):
		status = 404
	case errors.Is(err, apperrors.ErrEventConcurrentUpdate):
		status = 409
	case errors.Is(err, apperrors.ErrEventInvalidVersion), errors.Is(err, apperrors.ErrInvalidEventTimeRange), errors.Is(err, apperrors.ErrTitleRequired), errors.Is(err, apperrors.ErrEventInvalidContent), errors.Is(err, apperrors.ErrNothingToUpdate):
		status = 400
	}
	if status != 500 {
		message = err.Error()
	}
	c.JSON(status, gin.H{"error": message})
	return true
}

func NewEventHandler(service event.EventService) *EventHandler {
	return &EventHandler{service: service}
}

func (h *EventHandler) CreateEvent(c *gin.Context) {
	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}
	customDates, err := parseCustomDates(req.CustomDates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid custom date"})
		return
	}
	command := event.CreateCommand{
		Title:       req.Title,
		Content:     req.Content,
		Color:       req.Color,
		StartsAt:    *req.StartsAt,
		EndsAt:      *req.EndsAt,
		RepeatMode:  req.RepeatMode,
		CustomDates: customDates,
	}
	item, err := h.service.Create(c.Request.Context(), command)
	if errors.Is(err, apperrors.ErrTitleRequired) ||
		errors.Is(err, apperrors.ErrEventInvalidContent) ||
		errors.Is(err, apperrors.ErrEventInvalidColor) ||
		errors.Is(err, apperrors.ErrInvalidEventTimeRange) ||
		errors.Is(err, apperrors.ErrInvalidRepeatMode) ||
		errors.Is(err, apperrors.ErrCustomDatesRequired) ||
		errors.Is(err, apperrors.ErrCustomDatesNotAllowed) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create event",
		})
		return
	}
	c.JSON(http.StatusCreated, item)
}
