package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"note/internal/auth"
	apperrors "note/internal/errors"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/router"
	"note/internal/todo"

	"gorm.io/gorm"
)

func completionFixture(t *testing.T) (*gorm.DB, todo.Service, model.Todo, model.User, model.User, time.Time) {
	t.Helper()
	db, owner, member, group := groupSchemaFixture(t)
	if err := db.Create(&[]model.GroupMember{{GroupID: group.ID, UserID: owner.ID}, {GroupID: group.ID, UserID: member.ID}}).Error; err != nil {
		t.Fatal(err)
	}
	service := todo.NewService(todo.NewGORMRepository(db))
	date := time.Date(2026, 10, 3, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	startsAt := date.Add(9 * time.Hour)
	item, err := service.Create(context.Background(), todo.CreateCommand{
		GroupID: group.ID, CreatorID: owner.ID, Title: "多人完成", StartsAt: &startsAt, RepeatMode: model.RepeatOnce,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, service, item, owner, member, date
}

func TestTodoCompletionPersonalStateAndHistory(t *testing.T) {
	db, service, item, owner, viewer, date := completionFixture(t)
	ctx := context.Background()
	set := func(userID uint, done bool) {
		t.Helper()
		if err := service.SetOccurrenceDone(ctx, item.ID, userID, date, done); err != nil {
			t.Fatal(err)
		}
	}
	list := func() []todo.CompletionUser {
		t.Helper()
		items, err := service.GetOccurrenceCompletions(ctx, item.ID, owner.ID, date)
		if err != nil {
			t.Fatal(err)
		}
		return items
	}
	if got := list(); got == nil || len(got) != 0 {
		t.Fatalf("empty list: %#v", got)
	}
	// 先完成 ID 较大的 viewer，验证用户查询不能按数组索引匹配时间。
	set(viewer.ID, true)
	first := list()[0]
	set(viewer.ID, true)
	set(owner.ID, true)
	got := list()
	if len(got) != 2 || got[0].User.ID != viewer.ID || got[1].User.ID != owner.ID ||
		!got[0].CompletedAt.Equal(first.CompletedAt) || got[0].CompletedAt.IsZero() {
		t.Fatalf("deduplication or user/time pairing: %#v", got)
	}
	for _, entry := range got {
		if entry.User.Email != "" || entry.User.PasswordHash != "" {
			t.Fatal("completion query includes private user fields")
		}
	}
	for _, user := range []model.User{owner, viewer} {
		occurrences, err := service.CalendarOccurrences(ctx, user.ID, date, date.AddDate(0, 0, 1))
		if err != nil || len(occurrences) != 1 || !occurrences[0].OccurrenceDone {
			t.Fatalf("calendar user %d: %#v %v", user.ID, occurrences, err)
		}
	}
	set(viewer.ID, false)
	set(viewer.ID, false)
	if got := list(); len(got) != 1 || got[0].User.ID != owner.ID {
		t.Fatalf("undo removed another user: %#v", got)
	}
	occurrences, err := service.CalendarOccurrences(ctx, viewer.ID, date, date.AddDate(0, 0, 1))
	if err != nil || len(occurrences) != 1 || occurrences[0].OccurrenceDone {
		t.Fatalf("another user's completion affects calendar: %#v %v", occurrences, err)
	}
	// 日期编辑保留旧完成历史，但禁止新增旧日期的完成记录。
	newStart := date.AddDate(0, 0, 1).Add(9 * time.Hour)
	if _, err := service.Patch(ctx, item.ID, owner.ID, todo.PatchCommand{StartsAt: &newStart, Version: item.Version}); err != nil {
		t.Fatal(err)
	}
	if got := list(); len(got) != 1 {
		t.Fatal("patch erased completion history")
	}
	if err := service.SetOccurrenceDone(ctx, item.ID, viewer.ID, date, true); !errors.Is(err, apperrors.ErrTodoOccurrenceNotFound) {
		t.Fatalf("new completion for removed date: %v", err)
	}
	set(owner.ID, false)
	if got := list(); got == nil || len(got) != 0 {
		t.Fatalf("last undo: %#v", got)
	}
	var raw string
	if err := db.Table("todo_completions").Select("records").Where("todo_id = ?", item.ID).Scan(&raw).Error; err != nil || raw != "[]" {
		t.Fatalf("empty JSON not saved: %q %v", raw, err)
	}
}

func TestTodoCompletionHTTPIdentityAndMembership(t *testing.T) {
	db, service, item, owner, viewer, date := completionFixture(t)
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	r := router.NewWithWeb(handler.NewTodoHandler(service), nil, nil, nil, nil, nil, nil, tokens, "")
	request := func(method string, userID uint, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if userID != 0 {
			token, err := tokens.Generate(userID)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: %d want %d: %s", method, path, res.Code, status, res.Body.String())
		}
		return res
	}
	base := fmt.Sprintf("/api/todos/%d/occurrences/%s", item.ID, date.Format(time.DateOnly))
	get := base + "/completions"
	patch := base
	request(http.MethodGet, 0, get, "", 401)
	request(http.MethodPatch, 0, patch, `{"done":true}`, 401)
	request(http.MethodPatch, viewer.ID, patch, `{}`, 400)
	request(http.MethodGet, viewer.ID, fmt.Sprintf("/api/todos/%d/occurrences/2026-02-30/completions", item.ID), "", 400)
	request(http.MethodGet, viewer.ID, "/api/todos/99999/occurrences/2026-10-03/completions", "", 404)
	request(http.MethodPatch, viewer.ID, fmt.Sprintf("/api/todos/%d/occurrences/2026-10-04", item.ID), `{"done":true}`, 404)
	res := request(http.MethodPatch, viewer.ID, patch, fmt.Sprintf(`{"done":true,"user_id":%d}`, owner.ID), 204)
	if res.Body.Len() != 0 {
		t.Fatal("PATCH should have no response body")
	}
	res = request(http.MethodGet, owner.ID, get, "", 200)
	var response handler.OccurrenceCompletionsResponse
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.TodoID != item.ID || response.CompletedCount != 1 || len(response.Users) != 1 || response.Users[0].ID != viewer.ID {
		t.Fatalf("client spoofed completion identity: %#v", response)
	}
	for _, forbidden := range []string{"occurs_on", "occurrence_done", "password", "email"} {
		if strings.Contains(res.Body.String(), forbidden) {
			t.Fatalf("unexpected response field: %s", forbidden)
		}
	}
	// 保留 Todo 授权和完成记录，删除群成员资格后仍必须拒绝访问。
	for _, user := range []model.User{owner, viewer} {
		if err := db.Where("group_id = ? AND user_id = ?", item.GroupID, user.ID).Delete(&model.GroupMember{}).Error; err != nil {
			t.Fatal(err)
		}
		request(http.MethodGet, user.ID, get, "", 403)
		request(http.MethodPatch, user.ID, patch, `{"done":false}`, 403)
		occurrences, err := service.CalendarOccurrences(context.Background(), user.ID, date, date.AddDate(0, 0, 1))
		if err != nil || len(occurrences) != 0 {
			t.Fatalf("calendar exposed former group's todos: %#v %v", occurrences, err)
		}
	}
}

func TestTodoCompletionConcurrentUsers(t *testing.T) {
	db, service, item, owner, viewer, date := completionFixture(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for iteration := 0; iteration < 8; iteration++ {
		start := make(chan struct{})
		results := make(chan error, 4)
		for _, userID := range []uint{owner.ID, viewer.ID, owner.ID, viewer.ID} {
			go func(userID uint) {
				<-start
				results <- service.SetOccurrenceDone(ctx, item.ID, userID, date, true)
			}(userID)
		}
		close(start)
		for i := 0; i < 4; i++ {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		items, err := service.GetOccurrenceCompletions(ctx, item.ID, owner.ID, date)
		if err != nil || len(items) != 2 || items[0].User.ID == items[1].User.ID {
			t.Fatalf("lost update or duplicate: %#v %v", items, err)
		}
		for _, userID := range []uint{owner.ID, viewer.ID} {
			if err := service.SetOccurrenceDone(ctx, item.ID, userID, date, false); err != nil {
				t.Fatal(err)
			}
		}
	}
}
