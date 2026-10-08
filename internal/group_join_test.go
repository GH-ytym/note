package internal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"note/internal/group"
	"note/internal/model"
	"note/internal/todo"
)

func TestGroupJoinInitializesMembersAndPermissions(t *testing.T) {
	for _, withTodos := range []bool{false, true} {
		t.Run(fmt.Sprintf("with_todos=%t", withTodos), func(t *testing.T) {
			db, owner, member, item := groupSchemaFixture(t)
			if err := db.Model(&item).Update("code", "ABC123").Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.GroupMember{GroupID: item.ID, UserID: owner.ID}).Error; err != nil {
				t.Fatal(err)
			}

			var todos []model.Todo
			if withTodos {
				startsAt := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
				content := "入群前已经存在的 Todo"
				// 第二条模拟该用户退群前创建、退群后仍保留的 Todo。
				for _, creatorID := range []uint{owner.ID, member.ID} {
					todos = append(todos, model.Todo{
						GroupID: item.ID, CreatorID: creatorID, Title: "已有 Todo",
						Content: &content, StartsAt: &startsAt, RepeatMode: model.RepeatOnce,
					})
				}
				if err := db.Omit("Group", "Creator", "Members").Create(&todos).Error; err != nil {
					t.Fatal(err)
				}
			}

			repo := group.NewGORMRepository(db)
			if _, _, err := repo.Join(context.Background(), member.ID, "ABC123"); err != nil {
				t.Fatalf("join: %v", err)
			}
			var memberships []model.GroupMember
			if err := db.Where("group_id = ? AND user_id = ?", item.ID, member.ID).
				Find(&memberships).Error; err != nil {
				t.Fatal(err)
			}
			if len(memberships) != 1 || memberships[0].JoinedAt.IsZero() {
				t.Fatalf("join must save one dated membership: %+v", memberships)
			}

			var permissions []model.TodoMember
			if err := db.Where("user_id = ?", member.ID).Order("todo_id ASC").
				Find(&permissions).Error; err != nil {
				t.Fatal(err)
			}
			if len(permissions) != len(todos) {
				t.Fatalf("saved %d roles for %d todos", len(permissions), len(todos))
			}
			for i, permission := range permissions {
				want := model.TodoViewer
				if todos[i].CreatorID == member.ID {
					want = model.TodoEditor
				}
				if permission.TodoID != todos[i].ID || permission.Role != want {
					t.Fatalf("unexpected permission: %+v, want role %s", permission, want)
				}
			}

			// 已在群内时重复接受邀请不能重置已经授予的 editor。
			if withTodos {
				if err := db.Model(&model.TodoMember{}).
					Where("todo_id = ? AND user_id = ?", todos[0].ID, member.ID).
					Update("role", model.TodoEditor).Error; err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := repo.Join(context.Background(), member.ID, "ABC123"); err != nil {
				t.Fatalf("repeat join: %v", err)
			}
			var count int64
			if err := db.Model(&model.GroupMember{}).
				Where("group_id = ? AND user_id = ?", item.ID, member.ID).
				Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("repeat join duplicated membership: count=%d err=%v", count, err)
			}
			if withTodos {
				var permission model.TodoMember
				if err := db.Where("todo_id = ? AND user_id = ?", todos[0].ID, member.ID).
					First(&permission).Error; err != nil || permission.Role != model.TodoEditor {
					t.Fatalf("repeat join changed editor role: %+v err=%v", permission, err)
				}
			}
		})
	}
}

func TestGroupJoinRollsBackWhenPermissionInsertFails(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	if err := db.Model(&item).Update("code", "ABC123").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.GroupMember{GroupID: item.ID, UserID: owner.ID}).Error; err != nil {
		t.Fatal(err)
	}
	startsAt := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	todoService := todo.NewService(todo.NewGORMRepository(db))
	if _, err := todoService.Create(context.Background(), todo.CreateCommand{
		GroupID: item.ID, CreatorID: owner.ID, Title: "保留原数据",
		StartsAt: &startsAt, RepeatMode: model.RepeatOnce,
	}); err != nil {
		t.Fatal(err)
	}
	// 只在临时测试库中拒绝新成员的权限写入，验证成员关系也会回滚。
	trigger := fmt.Sprintf(`CREATE TEMP TRIGGER reject_join_permission
		BEFORE INSERT ON todo_members WHEN NEW.user_id = %d
		BEGIN SELECT RAISE(ABORT, 'test permission failure'); END`, member.ID)
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := group.NewGORMRepository(db).Join(context.Background(), member.ID, "ABC123"); err == nil {
		t.Fatal("join succeeded despite failed permission insertion")
	}
	for _, table := range []any{&model.GroupMember{}, &model.TodoMember{}} {
		var count int64
		if err := db.Model(table).Where("user_id = ?", member.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("join left partial data in %T: count=%d err=%v", table, count, err)
		}
	}
	for _, table := range []any{&model.GroupMember{}, &model.TodoMember{}, &model.Todo{}} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("failed join changed existing %T: count=%d err=%v", table, count, err)
		}
	}
}
