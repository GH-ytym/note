package internal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"note/internal/auth"
	"note/internal/handler"
	"note/internal/middleware"
	"note/internal/model"
	"note/internal/todo"

	"github.com/gin-gonic/gin"
)

func TestGroupTodoCreationPermissionsAndRollback(t *testing.T) {
	db, owner, member, group := groupSchemaFixture(t)
	if err := db.Create(&[]model.GroupMember{
		{GroupID: group.ID, UserID: owner.ID},
		{GroupID: group.ID, UserID: member.ID},
	}).Error; err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := handler.NewTodoHandler(todo.NewService(todo.NewGORMRepository(db)))
	r := gin.New()
	r.POST("/api/todos", middleware.RequireLogin(tokens), h.CreateTodo)
	request := func(userID uint, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", "/api/todos", strings.NewReader(body))
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
		return res
	}
	// member 是第二条群成员记录；也能创建，并且不能伪造创建者。
	body := `{"group_id":1,"creator_id":999,"title":"活动","starts_at":"2026-10-03T09:00:00+08:00","repeat_mode":"once"}`
	res := request(member.ID, body)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var item model.Todo
	if err := db.Preload("Members").First(&item).Error; err != nil {
		t.Fatal(err)
	}
	if item.GroupID != group.ID || item.CreatorID != member.ID || len(item.Members) != 2 {
		t.Fatalf("unexpected todo: %#v", item)
	}
	for _, permission := range item.Members {
		want := model.TodoViewer
		if permission.UserID == member.ID {
			want = model.TodoEditor
		}
		if permission.Role != want {
			t.Fatalf("user %d role %q want %q", permission.UserID, permission.Role, want)
		}
	}
	// 已存在的 editor 在重启迁移后不应被降级，内容也不应丢失。
	if err := db.Model(&model.TodoMember{}).Where("todo_id = ? AND user_id = ?", item.ID, owner.ID).Update("role", model.TodoEditor).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	var permission model.TodoMember
	if err := db.Where("todo_id = ? AND user_id = ?", item.ID, owner.ID).First(&permission).Error; err != nil || permission.Role != model.TodoEditor {
		t.Fatalf("migration changed permission: %#v %v", permission, err)
	}
	if res := request(0, body); res.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create: %d", res.Code)
	}
	if res := request(member.ID, strings.Replace(body, `"group_id":1`, `"group_id":999`, 1)); res.Code != http.StatusForbidden {
		t.Fatalf("outside group create: %d %s", res.Code, res.Body.String())
	}
	// 权限插入失败时，Todo 和自定义日期必须一起回滚。
	if err := db.Exec(`CREATE TEMP TRIGGER reject_todo_permission BEFORE INSERT ON todo_members BEGIN SELECT RAISE(ABORT, 'test permission failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	custom := `{"group_id":1,"title":"回滚","starts_at":"2026-10-03T09:00:00+08:00","repeat_mode":"custom","custom_dates":["2026-10-03"]}`
	if res := request(member.ID, custom); res.Code != http.StatusInternalServerError {
		t.Fatalf("failed insert: %d %s", res.Code, res.Body.String())
	}
	var todos, dates, permissions int64
	db.Model(&model.Todo{}).Count(&todos)
	db.Model(&model.TodoDate{}).Count(&dates)
	db.Model(&model.TodoMember{}).Count(&permissions)
	if todos != 1 || dates != 0 || permissions != 2 {
		t.Fatalf("partial commit: todos=%d dates=%d permissions=%d", todos, dates, permissions)
	}
}

func TestTodoMigrationRejectsUnassignedLegacyData(t *testing.T) {
	db := authTestDB(t)
	if err := db.Exec(`CREATE TABLE todos (id integer PRIMARY KEY, content text NOT NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO todos VALUES (1, '保留旧内容')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err == nil || !strings.Contains(err.Error(), "回填") {
		t.Fatalf("expected ownership migration error, got %v", err)
	}
	var content string
	if err := db.Raw("SELECT content FROM todos WHERE id = 1").Scan(&content).Error; err != nil || content != "保留旧内容" {
		t.Fatalf("legacy data changed: %q %v", content, err)
	}
}
