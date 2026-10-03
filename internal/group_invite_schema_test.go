package internal

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	apperrors "note/internal/errors"
	"note/internal/group"
	"note/internal/model"
	"note/internal/todo"
)

func TestGroupInviteMigrationDropsUniqueIndexAndPreservesData(t *testing.T) {
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
	if err := db.Exec("CREATE UNIQUE INDEX idx_groups_code ON groups(code)").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateDatabase(db); err != nil {
			t.Fatal(err)
		}
	}
	if db.Migrator().HasIndex(&model.Group{}, "idx_groups_code") {
		t.Fatal("legacy unique invite index remains")
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

	repo := group.NewGORMRepository(db)
	if err := repo.ReplaceInviteCode(context.Background(), first.ID, member.ID, second.Code); !errors.Is(err, apperrors.ErrGroupAccessDenied) {
		t.Fatalf("non-owner refreshed invite: %v", err)
	}
	if err := repo.ReplaceInviteCode(context.Background(), first.ID, owner.ID, "ABC123"); !errors.Is(err, apperrors.ErrGroupInviteCodeConflict) {
		t.Fatalf("same code should request regeneration: %v", err)
	}
	// 可以刷新成其他群的当前邀请码。
	if err := repo.ReplaceInviteCode(context.Background(), first.ID, owner.ID, second.Code); err != nil {
		t.Fatalf("different groups cannot share invite code: %v", err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Group{}).Where("code = ?", second.Code).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("duplicate codes not preserved: %d %v", count, err)
	}
	// 更新失败必须保留原邀请码。
	if err := db.Exec(`CREATE TRIGGER reject_invite_refresh BEFORE UPDATE OF code ON groups
		BEGIN SELECT RAISE(ABORT, 'test refresh failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceInviteCode(context.Background(), first.ID, owner.ID, "NEW123"); err == nil {
		t.Fatal("expected refresh failure")
	}
	code, err := repo.GetInviteCode(context.Background(), first.ID, member.ID)
	if err != nil || code != second.Code {
		t.Fatalf("failed refresh destroyed previous code: %q %v", code, err)
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
