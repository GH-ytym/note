package handler

import (
	"errors"
	"net/http"

	apperrors "note/internal/errors"
	"note/internal/event"

	"github.com/gin-gonic/gin"
)

// EventHandler translates Event HTTP requests into Event service calls.
type EventHandler struct {
	service event.EventService
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
	command := event.CreateCommand{
		Title:    req.Title,
		Content:  req.Content,
		Color:    req.Color,
		StartsAt: *req.StartsAt,
		EndsAt:   *req.EndsAt,
	}
	item, err := h.service.Create(c.Request.Context(), command)
	if errors.Is(err, apperrors.ErrTitleRequired) ||
		errors.Is(err, apperrors.ErrEventInvalidContent) ||
		errors.Is(err, apperrors.ErrEventInvalidColor) ||
		errors.Is(err, apperrors.ErrInvalidEventTimeRange) {
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
