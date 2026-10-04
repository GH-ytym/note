package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"note/internal/calendar"
	"note/internal/event"
	"note/internal/middleware"
	"note/internal/model"
	"note/internal/todo"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type calendarStub struct{ actorID uint }

func (s *calendarStub) Get(_ context.Context, actor uint, _, _ time.Time) (calendar.Result, error) {
	s.actorID = actor
	return calendar.Result{
		Todos:  []todo.CalendarOccurrence{{TodoID: 1, GroupID: 10, MyRole: model.TodoViewer}, {TodoID: 2, GroupID: 20, MyRole: model.TodoEditor}},
		Events: []event.CalendarOccurrence{{EventID: 1, GroupID: 20}, {EventID: 2, GroupID: 10}},
	}, nil
}

func TestCalendarGroupFilterKeepsAuthenticatedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &calendarStub{}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(middleware.UserIDKey, uint(7)); c.Next() })
	r.GET("/calendar", NewCalendarHandler(service).GetCalendar)
	for _, tc := range []struct {
		suffix string
		code   int
		count  int
	}{
		{"", 200, 2}, {"&group_id=10&user_id=99", 200, 1}, {"&group_id=999", 200, 0}, {"&group_id=-1", 400, 0},
	} {
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest("GET", "/calendar?from=2026-10-01&to=2026-11-01"+tc.suffix, nil))
		if res.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.suffix, res.Code, res.Body.String())
		}
		if tc.code != 200 {
			continue
		}
		var result struct {
			Data calendar.Result `json:"data"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if service.actorID != 7 || len(result.Data.Todos) != tc.count || len(result.Data.Events) != tc.count {
			t.Fatalf("%s: %+v", tc.suffix, result)
		}
		if tc.count == 1 && (result.Data.Todos[0].GroupID != 10 || result.Data.Events[0].GroupID != 10) {
			t.Fatal("mixed group calendar")
		}
	}
}

func TestDetailPermissionsForCurrentActor(t *testing.T) {
	item := model.Todo{ID: 1, CreatorID: 1, Members: []model.TodoMember{{UserID: 1, Role: model.TodoEditor}, {UserID: 2, Role: model.TodoViewer}, {UserID: 3, Role: model.TodoEditor}}}
	eventItem := model.Event{ID: 1, CreatorID: 1, Members: []model.EventMember{{UserID: 1, Role: model.EventEditor}, {UserID: 2, Role: model.EventViewer}, {UserID: 3, Role: model.EventEditor}}}
	for _, tc := range []struct {
		actor uint
		role  string
		count int
	}{{1, "editor", 3}, {2, "viewer", 0}, {3, "editor", 0}} {
		todoResponse := newTodoDetailResponse(item, tc.actor)
		eventResponse := newEventDetailResponse(eventItem, tc.actor)
		if string(todoResponse.MyRole) != tc.role || len(todoResponse.MemberRoles) != tc.count || string(eventResponse.MyRole) != tc.role || len(eventResponse.MemberRoles) != tc.count {
			t.Fatalf("actor %d: %+v, %+v", tc.actor, todoResponse, eventResponse)
		}
	}
}
