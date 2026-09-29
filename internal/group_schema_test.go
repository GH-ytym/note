package internal

import (
	"errors"
	"testing"

	"note/internal/model"

	"gorm.io/gorm"
)

// 走真正的启动迁移，而不是在测试里单独建表，避免漏接 migrateDatabase。
func groupSchemaFixture(t *testing.T) (*gorm.DB, model.User, model.User, model.Group) {
	t.Helper()
	db := authTestDB(t)
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []any{&model.Group{}, &model.GroupMember{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table for %T", table)
		}
	}
	owner := model.User{Username: "owner", Suffix: 12345, Email: "owner@example.com", PasswordHash: "test-hash", Nickname: "群主"}
	member := model.User{Username: "member", Suffix: 12345, Email: "member@example.com", PasswordHash: "test-hash", Nickname: "成员"}
	for _, user := range []*model.User{&owner, &member} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	group := model.Group{Name: "学习群", OwnerID: owner.ID}
	if err := db.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	return db, owner, member, group
}

func TestGroupSchemaMembershipAndPreload(t *testing.T) {
	db, owner, member, first := groupSchemaFixture(t)
	// 群名不必唯一，群的身份由 ID 区分。
	second := model.Group{Name: first.Name, OwnerID: owner.ID}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("same group name should be allowed: %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("groups must have different IDs")
	}
	memberships := []model.GroupMember{
		{GroupID: first.ID, UserID: owner.ID},
		{GroupID: first.ID, UserID: member.ID},
		{GroupID: second.ID, UserID: owner.ID},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if memberships[0].JoinedAt.IsZero() {
		t.Fatal("membership join time was not populated")
	}
	duplicate := model.GroupMember{GroupID: first.ID, UserID: owner.ID}
	if err := db.Create(&duplicate).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("duplicate membership: got %v, want duplicated key", err)
	}

	// 重复启动不能丢失已有群资料、成员资格和加入时间。
	if err := migrateDatabase(db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	var loaded model.Group
	if err := db.Preload("Owner").Preload("Members.User").First(&loaded, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.Name != first.Name || loaded.Owner == nil || loaded.Owner.ID != owner.ID {
		t.Fatalf("unexpected group or owner: %+v", loaded)
	}
	if len(loaded.Members) != 2 {
		t.Fatalf("got %d members, want 2", len(loaded.Members))
	}
	seen := make(map[uint]bool)
	for _, entry := range loaded.Members {
		if entry.GroupID != first.ID || entry.User == nil || entry.User.ID != entry.UserID {
			t.Fatalf("invalid member association: %+v", entry)
		}
		seen[entry.UserID] = true
		if entry.UserID == owner.ID && !entry.JoinedAt.Equal(memberships[0].JoinedAt) {
			t.Fatal("repeat migration changed join time")
		}
	}
	if !seen[owner.ID] || !seen[member.ID] {
		t.Fatal("members missing after migration")
	}
	var count int64
	if err := db.Model(&model.GroupMember{}).Where("user_id = ?", owner.ID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("one user must be able to join two groups: count=%d err=%v", count, err)
	}
}

func TestGroupSchemaForeignKeysAndDeletion(t *testing.T) {
	db, owner, member, group := groupSchemaFixture(t)
	// 验证约束真的存在并启用，而不只是结构体上写了标签。
	for _, tc := range []struct {
		name string
		row  any
	}{
		{"unknown owner", &model.Group{Name: "无效群", OwnerID: 999999}},
		{"unknown group", &model.GroupMember{GroupID: 999999, UserID: member.ID}},
		{"unknown user", &model.GroupMember{GroupID: group.ID, UserID: 999999}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Create(tc.row).Error; err == nil {
				t.Fatal("invalid foreign key accepted")
			}
		})
	}
	// 此时群主尚无成员记录，删除仍应被 groups.owner_id 的外键阻止。
	if err := db.Delete(&owner).Error; err == nil {
		t.Fatal("deleted user who still owns a group")
	}
	memberships := []model.GroupMember{
		{GroupID: group.ID, UserID: owner.ID},
		{GroupID: group.ID, UserID: member.ID},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&member).Error; err == nil {
		t.Fatal("deleted user who still belongs to a group")
	}
	if err := db.Delete(&group).Error; err != nil {
		t.Fatalf("delete group: %v", err)
	}
	var count int64
	if err := db.Model(&model.GroupMember{}).Where("group_id = ?", group.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("membership cascade failed: count=%d err=%v", count, err)
	}
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("deleting group must preserve accounts: count=%d err=%v", count, err)
	}
}
