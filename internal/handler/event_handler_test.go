package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	apperrors "note/internal/errors"
	"note/internal/event"
	"note/internal/model"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type mockEventService struct {
	createCalls int
	received    event.CreateCommand
	result      model.Event
	createErr   error
}

func (m *mockEventService) Get(_ context.Context, _ uint) (model.Event, error) {
	return m.result, m.createErr
}
func (m *mockEventService) Patch(_ context.Context, _ uint, _ event.PatchCommand) (model.Event, error) {
	return m.result, m.createErr
}

func (m *mockEventService) Create(
	_ context.Context,
	cmd event.CreateCommand,
) (model.Event, error) {
	m.createCalls++
	m.received = cmd
	return m.result, m.createErr
}

func (m *mockEventService) ListInRange(
	_ context.Context,
	_ time.Time,
	_ time.Time,
) ([]event.CalendarOccurrence, error) {
	return nil, nil
}

func TestCreateEventHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockEventService{
		result: model.Event{
			ID:    1,
			Title: "项目会议",
		},
	}
	eventHandler := NewEventHandler(service)

	router := gin.New()
	router.POST("/events", eventHandler.CreateEvent)

	body := `{
		"title": "项目会议",
		"content": "讨论 Event 功能",
		"color": "#AABBCC",
		"starts_at": "2026-09-03T10:00:00+08:00",
		"ends_at": "2026-09-03T11:00:00+08:00",
		"repeat_mode": "custom",
		"custom_dates": ["2026-09-05"]
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/events",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			response.Code,
			http.StatusCreated,
			response.Body.String(),
		)
	}

	if service.createCalls != 1 {
		t.Fatalf(
			"Service Create calls = %d, want 1",
			service.createCalls,
		)
	}

	if service.received.Title != "项目会议" {
		t.Errorf(
			"command Title = %q, want %q",
			service.received.Title,
			"项目会议",
		)
	}

	if service.received.Content == nil {
		t.Fatal("command Content is nil")
	}

	if *service.received.Content != "讨论 Event 功能" {
		t.Errorf(
			"command Content = %q, want %q",
			*service.received.Content,
			"讨论 Event 功能",
		)
	}

	if service.received.Color != "#AABBCC" {
		t.Errorf(
			"command Color = %q, want %q",
			service.received.Color,
			"#AABBCC",
		)
	}

	if got := service.received.StartsAt.Format(time.RFC3339); got != "2026-09-03T10:00:00+08:00" {
		t.Errorf("command StartsAt = %q", got)
	}
	if got := service.received.EndsAt.Format(time.RFC3339); got != "2026-09-03T11:00:00+08:00" {
		t.Errorf("command EndsAt = %q", got)
	}
	if service.received.RepeatMode != model.RepeatCustom {
		t.Errorf("command RepeatMode = %q, want %q", service.received.RepeatMode, model.RepeatCustom)
	}
	if len(service.received.CustomDates) != 1 || service.received.CustomDates[0].Format(time.DateOnly) != "2026-09-05" {
		t.Errorf("command CustomDates = %v, want [2026-09-05]", service.received.CustomDates)
	}
}

func TestCreateEventHandlerRejectsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "malformed JSON",
			body: `{`,
		},
		{
			name: "missing required fields",
			body: `{}`,
		},
		{
			name: "invalid start time",
			body: `{
				"title":"项目会议",
				"starts_at":"not-a-time",
				"ends_at":"2026-09-03T11:00:00+08:00"
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &mockEventService{}
			response := performCreateEventRequest(service, test.body)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					response.Code,
					http.StatusBadRequest,
					response.Body.String(),
				)
			}
			if service.createCalls != 0 {
				t.Errorf("Service Create calls = %d, want 0", service.createCalls)
			}
		})
	}
}

func TestCreateEventHandlerMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "title required",
			serviceErr: apperrors.ErrTitleRequired,
			wantStatus: http.StatusBadRequest,
			wantBody:   apperrors.ErrTitleRequired.Error(),
		},
		{
			name:       "invalid content",
			serviceErr: apperrors.ErrEventInvalidContent,
			wantStatus: http.StatusBadRequest,
			wantBody:   apperrors.ErrEventInvalidContent.Error(),
		},
		{
			name:       "invalid color",
			serviceErr: apperrors.ErrEventInvalidColor,
			wantStatus: http.StatusBadRequest,
			wantBody:   apperrors.ErrEventInvalidColor.Error(),
		},
		{
			name:       "invalid time range",
			serviceErr: apperrors.ErrInvalidEventTimeRange,
			wantStatus: http.StatusBadRequest,
			wantBody:   apperrors.ErrInvalidEventTimeRange.Error(),
		},
		{
			name:       "unexpected repository error",
			serviceErr: errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "failed to create event",
		},
	}

	validBody := `{
		"title":"项目会议",
		"content":"讨论 Event 功能",
		"color":"#AABBCC",
		"starts_at":"2026-09-03T10:00:00+08:00",
		"ends_at":"2026-09-03T11:00:00+08:00",
		"repeat_mode":"once"
	}`

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &mockEventService{createErr: test.serviceErr}
			response := performCreateEventRequest(service, validBody)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					response.Code,
					test.wantStatus,
					response.Body.String(),
				)
			}
			if !strings.Contains(response.Body.String(), test.wantBody) {
				t.Errorf("body = %q, want it to contain %q", response.Body.String(), test.wantBody)
			}
			if service.createCalls != 1 {
				t.Errorf("Service Create calls = %d, want 1", service.createCalls)
			}
		})
	}
}

func performCreateEventRequest(
	service event.EventService,
	body string,
) *httptest.ResponseRecorder {
	eventHandler := NewEventHandler(service)
	router := gin.New()
	router.POST("/events", eventHandler.CreateEvent)

	request := httptest.NewRequest(
		http.MethodPost,
		"/events",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
