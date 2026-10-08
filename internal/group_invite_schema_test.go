package internal

import (
	"context"
	"regexp"
	"testing"
	"time"

	"note/internal/model"
	"note/internal/todo"
)

func TestGroupInviteMigrationPreservesUniqueIndexAndPreservesData(t *testing.T) {
	db, owner, member, first := groupSchemaFixture(t)
	if err := db.Model(&first).Update("code", "ABC123").Error; err != nil {
		t.Fatal(err)
	}
	second := model.Group{Name: "另一个群", OwnerID: owner.ID, Code: "Q7W8E9"}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.GroupMember{
		{GroupID: first.ID, UserID: owner.ID},
		{GroupID: first.ID, UserID: member.ID},
		{GroupID: second.ID, UserID: owner.ID},
	}).Error; err != nil {
		t.Fatal(err)
	}
	startsAt := time.Now().UTC()
	item, err := todo.NewService(todo.NewGORMRepository(db)).Create(context.Background(), todo.CreateCommand{
		GroupID: first.ID, CreatorID: owner.ID, Title: "保留群内 Todo", StartsAt: &startsAt, RepeatMode: model.RepeatOnce,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 模拟旧版本留下的真实唯一索引，而不是只检查模型标签。
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_code ON groups(code)").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateDatabase(db); err != nil {
			t.Fatal(err)
		}
	}
	if !db.Migrator().HasIndex(&model.Group{}, "idx_groups_code") {
		t.Fatal("unique group code index missing")
	}
	var saved model.Group
	if err := db.First(&saved, first.ID).Error; err != nil || saved.Code != "ABC123" {
		t.Fatalf("migration changed existing invite: %#v %v", saved, err)
	}
	var memberCount int64
	if err := db.Model(&model.GroupMember{}).Count(&memberCount).Error; err != nil || memberCount != 3 {
		t.Fatalf("migration lost group members: %d %v", memberCount, err)
	}
	var savedTodo model.Todo
	if err := db.Preload("Members").First(&savedTodo, item.ID).Error; err != nil || savedTodo.Title != item.Title || len(savedTodo.Members) != 2 {
		t.Fatalf("migration changed group todo: %#v %v", savedTodo, err)
	}

	duplicate := model.Group{Name: "duplicate", OwnerID: owner.ID, Code: first.Code}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate group code accepted")
	}

}

func TestGroupInviteMigrationBackfillsLegacyGroups(t *testing.T) {
	db := authTestDB(t)
	if err := migrateAuthSchema(db); err != nil {
		t.Fatal(err)
	}
	owner := model.User{Username: "legacy-owner", Suffix: 12345, Email: "legacy@example.com", PasswordHash: "test-hash"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE groups (
		id integer PRIMARY KEY AUTOINCREMENT, name text NOT NULL, owner_id integer NOT NULL,
		created_at datetime, updated_at datetime,
		FOREIGN KEY(owner_id) REFERENCES users(id))`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO groups(name, owner_id) VALUES (?, ?), (?, ?)", "旧群一", owner.ID, "旧群二", owner.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	var groups []model.Group
	if err := db.Order("id").Find(&groups).Error; err != nil || len(groups) != 2 {
		t.Fatalf("legacy groups missing: %#v %v", groups, err)
	}
	pattern := regexp.MustCompile(`^[0-9A-Z]{6}$`)
	for _, item := range groups {
		if !pattern.MatchString(item.Code) || item.OwnerID != owner.ID {
			t.Fatalf("invalid migrated group: %#v", item)
		}
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	for _, original := range groups {
		var saved model.Group
		if err := db.First(&saved, original.ID).Error; err != nil || saved.Code != original.Code {
			t.Fatalf("repeat migration changed invite: %#v %v", saved, err)
		}
	}
}
