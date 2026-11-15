package internal

import (
	"context"
	"note/internal/auth"
	"note/internal/group"
	"note/internal/model"
	"testing"
)

func TestPersonalGroupCreationAndBackfill(t *testing.T) {
	db, owner, _, _ := groupSchemaFixture(t)
	user := model.User{Username: "personaltest", Suffix: 12345, Email: "personal@example.com", PasswordHash: "test"}
	if err := auth.NewGORMRepository(db).Create(context.Background(), &user); err != nil {
		t.Fatal(err)
	}
	check := func(id uint) model.Group {
		t.Helper()
		var rows []model.Group
		if err := db.Where("owner_id = ? AND policy = ?", id, model.Personal).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("personal groups for %d: %d", id, len(rows))
		}
		var n int64
		db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", rows[0].ID, id).Count(&n)
		if n != 1 {
			t.Fatalf("membership count: %d", n)
		}
		return rows[0]
	}
	item := check(user.ID)
	for i := 0; i < 2; i++ {
		if err := migrateDatabase(db); err != nil {
			t.Fatal(err)
		}
	}
	check(owner.ID)
	check(user.ID)
	repo := group.NewGORMRepository(db)
	if err := repo.Dismiss(context.Background(), item.ID, user.ID); err == nil {
		t.Fatal("personal group dismissed")
	}
	if err := repo.Quit(context.Background(), item.ID, user.ID, nil); err == nil {
		t.Fatal("personal group quit")
	}
}
func TestPersonalGroupFailureRollsBackUser(t *testing.T) {
	db, _, _, _ := groupSchemaFixture(t)
	if err := db.Exec(`CREATE TRIGGER fail_personal BEFORE INSERT ON groups WHEN NEW.policy = 'personal' BEGIN SELECT RAISE(ABORT, 'test failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "rollbackpersonal", Suffix: 12345, Email: "rollbackpersonal@example.com", PasswordHash: "test"}
	if err := auth.NewGORMRepository(db).Create(context.Background(), &user); err == nil {
		t.Fatal("expected failure")
	}
	for _, table := range []any{&model.User{}, &model.NotificationState{}} {
		var n int64
		q := db.Model(table)
		if _, ok := table.(*model.User); ok {
			q = q.Where("email = ?", user.Email)
		} else {
			q = q.Where("user_id = ?", user.ID)
		}
		if err := q.Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("transaction left %T", table)
		}
	}
}
