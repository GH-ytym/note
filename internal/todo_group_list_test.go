package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"note/internal/auth"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/router"
	"note/internal/todo"
)

func TestTodoListCurrentGroupMembership(t *testing.T) {
	db, owner, outsider, item := groupSchemaFixture(t)
	if err := db.Create(&model.GroupMember{GroupID: item.ID, UserID: owner.ID}).Error; err != nil {
		t.Fatal(err)
	}
	service := todo.NewService(todo.NewGORMRepository(db))
	startsAt := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	if _, err := service.Create(context.Background(), todo.CreateCommand{
		GroupID: item.ID, CreatorID: owner.ID, Title: "群内列表",
		StartsAt: &startsAt, RepeatMode: model.RepeatOnce,
	}); err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	r := router.NewWithWeb(handler.NewTodoHandler(service), nil, nil, nil, nil, nil, tokens, "")
	request := func(userID uint, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/groups/%d/todos", item.ID), nil)
		if userID != 0 {
			token, err := tokens.Generate(userID)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("user %d: got %d, want %d: %s", userID, res.Code, want, res.Body.String())
		}
		return res
	}
	res := request(owner.ID, http.StatusOK)
	var page struct {
		Data  []model.Todo `json:"data"`
		Total int64        `json:"total"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Total != 1 || page.Data[0].GroupID != item.ID {
		t.Fatalf("incorrect member list: %+v", page)
	}
	request(0, http.StatusUnauthorized)
	request(outsider.ID, http.StatusForbidden)

	// 即使旧 editor 授权仍在，也不能代替当前群成员资格。
	if err := db.Where("group_id = ? AND user_id = ?", item.ID, owner.ID).
		Delete(&model.GroupMember{}).Error; err != nil {
		t.Fatal(err)
	}
	request(owner.ID, http.StatusForbidden)
}
