package handler

import (
	"errors"
	"net/http"
	"strconv"

	apperrors "note/internal/errors"
	"note/internal/event"
	"note/internal/middleware"
	"note/internal/model"

	"github.com/gin-gonic/gin"
)

// EventHandler translates Event HTTP requests into Event service calls.
type EventHandler struct {
	service event.Service
}

func (h *EventHandler) GetEvent(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if eventResponseError(c, eventIdentityError(userID)) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "invalid event id"})
		return
	}
	item, err := h.service.Get(c.Request.Context(), uint(id), userID)
	if eventResponseError(c, err) {
		return
	}
	c.JSON(200, newEventDetailResponse(item))
}

func (h *EventHandler) PatchEvent(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if eventResponseError(c, eventIdentityError(userID)) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "invalid event id"})
		return
	}
	var req PatchEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	item, err := h.service.Patch(c.Request.Context(), uint(id), userID, event.PatchCommand{Title: req.Title, Content: req.Content, StartsAt: req.StartsAt, EndsAt: req.EndsAt, Version: req.Version})
	if eventResponseError(c, err) {
		return
	}
	c.JSON(200, newEventDetailResponse(item))
}

func eventResponseError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	status := http.StatusInternalServerError
	message := "failed to read or update event"
	switch {
	case errors.Is(err, apperrors.ErrGroupUnauthenticated):
		status = 401
	case errors.Is(err, apperrors.ErrGroupAccessDenied), errors.Is(err, apperrors.ErrEventEditDenied), errors.Is(err, apperrors.ErrEventDeleteDenied), errors.Is(err, apperrors.ErrEventPermissionDenied):
		status = 403
	case errors.Is(err, apperrors.ErrEventNotFound), errors.Is(err, apperrors.ErrEventMemberNotFound):
		status = 404
	case errors.Is(err, apperrors.ErrEventConcurrentUpdate):
		status = 409
	case errors.Is(err, apperrors.ErrEventInvalidVersion), errors.Is(err, apperrors.ErrInvalidEventTimeRange), errors.Is(err, apperrors.ErrTitleRequired), errors.Is(err, apperrors.ErrEventInvalidContent), errors.Is(err, apperrors.ErrNothingToUpdate):
		status = 400
	case errors.Is(err, apperrors.ErrEventInvalidGroup), errors.Is(err, apperrors.ErrEventRoleInvalid), errors.Is(err, apperrors.ErrInvalidPagination), errors.Is(err, apperrors.ErrEventInvalidColor), errors.Is(err, apperrors.ErrInvalidRepeatMode), errors.Is(err, apperrors.ErrCustomDatesRequired), errors.Is(err, apperrors.ErrCustomDatesNotAllowed):
		status = 400
	}
	if status != 500 {
		message = err.Error()
	}
	c.JSON(status, gin.H{"error": message})
	return true
}

func NewEventHandler(service event.Service) *EventHandler {
	return &EventHandler{service: service}
}

func (h *EventHandler) CreateEvent(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if eventResponseError(c, eventIdentityError(userID)) {
		return
	}
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
		GroupID:     req.GroupID,
		CreatorID:   userID,
		Title:       req.Title,
		Content:     req.Content,
		Color:       req.Color,
		StartsAt:    *req.StartsAt,
		EndsAt:      *req.EndsAt,
		RepeatMode:  req.RepeatMode,
		CustomDates: customDates,
	}
	item, err := h.service.Create(c.Request.Context(), command)
	if eventResponseError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, newEventDetailResponse(item))
}

func eventIdentityError(userID uint) error {
	if userID == 0 {
		return apperrors.ErrGroupUnauthenticated
	}
	return nil
}

func (h *EventHandler) ListEvents(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if eventResponseError(c, eventIdentityError(userID)) {
		return
	}
	var uri GroupEventsURI
	var query ListEventsQuery
	if c.ShouldBindUri(&uri) != nil || c.ShouldBindQuery(&query) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	page, err := h.service.List(c.Request.Context(), event.ListQuery{GroupID: uri.GroupID, UserID: userID, Page: query.Page, PageSize: query.PageSize})
	if eventResponseError(c, err) {
		return
	}
	items := make([]EventDetailResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, newEventDetailResponse(item))
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "page": page.Page, "page_size": page.PageSize, "total": page.Total})
}

func (h *EventHandler) DeleteEvent(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if eventResponseError(c, eventIdentityError(userID)) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event id"})
		return
	}
	if eventResponseError(c, h.service.Delete(c.Request.Context(), uint(id), userID)) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *EventHandler) PatchRoles(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if eventResponseError(c, eventIdentityError(userID)) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event id"})
		return
	}
	// 沿用 Todo DTO：1 设置为 editor，2 设置为 viewer。
	var req RolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	role := model.EventEditor
	if req.Role == 2 {
		role = model.EventViewer
	}
	if eventResponseError(c, h.service.PatchRole(c.Request.Context(), userID, uint(id), req.UserIDs, role)) {
		return
	}
	c.Status(http.StatusNoContent)
}
