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

	"github.com/gin-gonic/gin"
)

func TestTodoGetCurrentGroupMembership(t *testing.T) {
	db, owner, creator, group := groupSchemaFixture(t)
	editor := model.User{Username: "editor", Suffix: 12345, Email: "editor@example.com", PasswordHash: "test-hash"}
	outsider := model.User{Username: "outsider", Suffix: 12345, Email: "outsider@example.com", PasswordHash: "test-hash"}
	for _, user := range []*model.User{&editor, &outsider} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&[]model.GroupMember{
		{GroupID: group.ID, UserID: owner.ID},
		{GroupID: group.ID, UserID: creator.ID},
		{GroupID: group.ID, UserID: editor.ID},
	}).Error; err != nil {
		t.Fatal(err)
	}
	otherGroup := model.Group{Name: "另一个群", OwnerID: outsider.ID}
	if err := db.Create(&otherGroup).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.GroupMember{GroupID: otherGroup.ID, UserID: outsider.ID}).Error; err != nil {
		t.Fatal(err)
	}

	service := todo.NewService(todo.NewGORMRepository(db))
	startsAt := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	date := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	item, err := service.Create(context.Background(), todo.CreateCommand{
		GroupID: group.ID, CreatorID: creator.ID, Title: "群内详情",
		StartsAt: &startsAt, RepeatMode: model.RepeatCustom, CustomDates: []time.Time{date},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.TodoMember{}).
		Where("todo_id = ? AND user_id = ?", item.ID, editor.ID).
		Update("role", model.TodoEditor).Error; err != nil {
		t.Fatal(err)
	}
	// 模拟退群后仍保留旧 editor 授权；它不能代替当前群成员资格。
	if err := db.Create(&model.TodoMember{TodoID: item.ID, UserID: outsider.ID, Role: model.TodoEditor}).Error; err != nil {
		t.Fatal(err)
	}

	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := handler.NewTodoHandler(service)
	r := router.NewWithWeb(h, nil, nil, nil, nil, nil, nil, tokens, "")
	request := func(userID uint, path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if userID != 0 {
			token, err := tokens.Generate(userID)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		return res
	}
	path := fmt.Sprintf("/api/todos/%d", item.ID)

	for _, tc := range []struct {
		name   string
		userID uint
	}{
		{"viewer", owner.ID}, {"editor", editor.ID}, {"creator", creator.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, prefix := range []string{"", "/api"} {
				res := request(tc.userID, fmt.Sprintf("%s/todos/%d", prefix, item.ID))
				if res.Code != http.StatusOK {
					t.Fatalf("get: %d %s", res.Code, res.Body.String())
				}
				var got handler.TodoDetailResponse
				if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.ID != item.ID || got.Title != item.Title || got.Content == nil || *got.Content != *item.Content ||
					got.GroupID != group.ID || got.CreatorID != creator.ID ||
					len(got.CustomDates) != 1 || !got.CustomDates[0].Date.Equal(date) {
					t.Fatalf("incomplete detail: %#v", got)
				}
				if got.Creator == nil || got.Creator.ID != creator.ID || got.Creator.Username != creator.Username ||
					got.Creator.Suffix != creator.Suffix || got.Creator.Nickname != creator.Nickname || got.Creator.Avatar != "" {
					t.Fatalf("incorrect creator profile: %#v", got.Creator)
				}
				if !strings.Contains(res.Body.String(), `"avatar":""`) {
					t.Fatal("response omits blank initial avatar")
				}
				if strings.Contains(res.Body.String(), `"members"`) || strings.Contains(strings.ToLower(res.Body.String()), "password") ||
					strings.Contains(strings.ToLower(res.Body.String()), "email") || strings.Contains(res.Body.String(), creator.PasswordHash) {
					t.Fatal("detail leaks member or password data")
				}
			}
		})
	}

	t.Run("repository loads only creator display fields", func(t *testing.T) {
		got, err := service.Get(context.Background(), item.ID, owner.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Creator == nil || got.Creator.ID != creator.ID || got.Creator.Username != creator.Username {
			t.Fatalf("creator association not loaded: %#v", got.Creator)
		}
		if got.Creator.Email != "" || got.Creator.PasswordHash != "" {
			t.Fatal("creator preload includes private account data")
		}
	})

	t.Run("current member without todo permission remains visible", func(t *testing.T) {
		if err := db.Where("todo_id = ? AND user_id = ?", item.ID, owner.ID).Delete(&model.TodoMember{}).Error; err != nil {
			t.Fatal(err)
		}
		if res := request(owner.ID, path); res.Code != http.StatusOK {
			t.Fatalf("group visibility depends on permission row: %d %s", res.Code, res.Body.String())
		}
	})

	for _, tc := range []struct {
		name   string
		userID uint
		path   string
		status int
	}{
		{"stale editor in another group", outsider.ID, path, http.StatusForbidden},
		{"query cannot spoof identity or group", outsider.ID, fmt.Sprintf("%s?user_id=%d&group_id=%d", path, creator.ID, otherGroup.ID), http.StatusForbidden},
		{"anonymous", 0, path, http.StatusUnauthorized},
		{"missing todo", creator.ID, "/api/todos/99999", http.StatusNotFound},
		{"invalid id", creator.ID, "/api/todos/abc", http.StatusBadRequest},
		{"zero id", creator.ID, "/api/todos/0", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := request(tc.userID, tc.path)
			if res.Code != tc.status {
				t.Fatalf("get: %d want %d: %s", res.Code, tc.status, res.Body.String())
			}
			if strings.Contains(res.Body.String(), item.Title) {
				t.Fatal("denied response leaks todo content")
			}
		})
	}

	for _, tc := range []struct {
		name   string
		userID uint
	}{
		{"creator after leaving", creator.ID}, {"group owner after leaving", owner.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Where("group_id = ? AND user_id = ?", group.ID, tc.userID).Delete(&model.GroupMember{}).Error; err != nil {
				t.Fatal(err)
			}
			if res := request(tc.userID, path); res.Code != http.StatusForbidden {
				t.Fatalf("former member can read: %d %s", res.Code, res.Body.String())
			}
			got, err := service.Get(context.Background(), item.ID, tc.userID)
			if !errors.Is(err, apperrors.ErrGroupAccessDenied) || got.ID != 0 || got.Title != "" {
				t.Fatalf("denied service read returns data: %#v %v", got, err)
			}
		})
	}
}

func TestTodoGetRequiresIdentityWithoutMiddleware(t *testing.T) {
	// 防止以后组装路由时漏挂中间件，或用 query 参数伪造登录身份。
	r := gin.New()
	r.GET("/todos/:id", handler.NewTodoHandler(nil).GetTodo)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/todos/1?user_id=1", nil))
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("get without identity: %d %s", res.Code, res.Body.String())
	}
}
