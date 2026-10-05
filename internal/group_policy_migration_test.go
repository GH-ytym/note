package internal

import (
	"testing"
	"time"

	"note/internal/model"
)

func TestGroupPolicyMigrationPreservesLegacyGroupData(t *testing.T) {
	db := authTestDB(t)
	if err := migrateAuthSchema(db); err != nil {
		t.Fatal(err)
	}
	owner := model.User{Username: "policy_owner", Suffix: 12345, Email: "policy@example.com", PasswordHash: "hash"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	// 模拟已有邀请码、尚无 policy 的旧 groups 表。
	if err := db.Exec(`CREATE TABLE groups (
		id integer PRIMARY KEY AUTOINCREMENT, name text NOT NULL,
		owner_id integer NOT NULL, code text NOT NULL,
		created_at datetime, updated_at datetime,
		FOREIGN KEY(owner_id) REFERENCES users(id) ON DELETE RESTRICT
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO groups(id, name, owner_id, code) VALUES (1, ?, ?, ?)", "旧群", owner.ID, "ABC123").Error; err != nil {
		t.Fatal(err)
	}
	// CreateTable 仅建指定子表，避免在准备旧库时提前升级 groups。
	for _, item := range []any{&model.GroupMember{}, &model.Todo{}, &model.TodoMember{}, &model.Event{}, &model.EventMember{}} {
		if err := db.Migrator().CreateTable(item); err != nil {
			t.Fatal(err)
		}
	}
	member := model.GroupMember{GroupID: 1, UserID: owner.ID}
	if err := db.Omit("User").Create(&member).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	content := "保留内容"
	todo := model.Todo{GroupID: 1, CreatorID: owner.ID, Title: "已有 Todo", Content: &content, StartsAt: &start,
		Members: []model.TodoMember{{UserID: owner.ID, Role: model.TodoEditor}}}
	if err := db.Omit("Group", "Creator", "Members.User").Create(&todo).Error; err != nil {
		t.Fatal(err)
	}
	event := model.Event{GroupID: 1, CreatorID: owner.ID, Title: "已有 Event", StartsAt: start, EndsAt: start.Add(time.Hour),
		Members: []model.EventMember{{UserID: owner.ID, Role: model.EventEditor}}}
	if err := db.Omit("Group", "Creator", "Members.User").Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	var saved model.Group
	if err := db.First(&saved, 1).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Policy != model.Public || saved.Name != "旧群" || saved.Code != "ABC123" || saved.OwnerID != owner.ID {
		t.Fatalf("unexpected migrated group: %+v", saved)
	}
	// 修改过的策略不能被下一次启动重置为 public。
	if err := db.Model(&saved).Update("policy", model.Approval).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Preload("Members").First(&saved, 1).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Policy != model.Approval || len(saved.Members) != 1 || !saved.Members[0].JoinedAt.Equal(member.JoinedAt) {
		t.Fatalf("repeat migration changed group or members: %+v", saved)
	}
	var savedTodo model.Todo
	if err := db.Preload("Members").First(&savedTodo, todo.ID).Error; err != nil {
		t.Fatal(err)
	}
	if savedTodo.Title != todo.Title || savedTodo.Content == nil || *savedTodo.Content != content || len(savedTodo.Members) != 1 || savedTodo.Members[0].Role != model.TodoEditor {
		t.Fatalf("migration changed todo or roles: %+v", savedTodo)
	}
	var savedEvent model.Event
	if err := db.Preload("Members").First(&savedEvent, event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if savedEvent.Title != event.Title || !savedEvent.EndsAt.Equal(event.EndsAt) || len(savedEvent.Members) != 1 || savedEvent.Members[0].Role != model.EventEditor {
		t.Fatalf("migration changed event or roles: %+v", savedEvent)
	}
	if err := db.Model(&saved).Update("policy", "invalid").Error; err == nil {
		t.Fatal("legacy database accepted invalid policy")
	}
}

func TestGroupPolicyFreshSchemaConstraints(t *testing.T) {
	db, owner, _, item := groupSchemaFixture(t)
	if item.Policy != model.Public {
		t.Fatalf("default policy = %q, want public", item.Policy)
	}
	for _, policy := range []model.GroupPolicy{model.Restricted, model.Public, model.Approval, model.Personal} {
		if err := db.Model(&item).Update("policy", policy).Error; err != nil {
			t.Fatalf("valid policy %q rejected: %v", policy, err)
		}
	}
	for _, policy := range []any{"invalid", "", nil} {
		if err := db.Model(&item).Update("policy", policy).Error; err == nil {
			t.Fatalf("invalid policy %#v accepted", policy)
		}
	}
	invalid := model.Group{Name: "无效策略", OwnerID: owner.ID, Policy: "invalid"}
	if err := db.Create(&invalid).Error; err == nil {
		t.Fatal("insert accepted invalid policy")
	}
}
