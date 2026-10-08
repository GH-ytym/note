package internal

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/todo"
)

func createTestTodo(
	t *testing.T,
	service todo.Service,
	groupID uint,
	creatorID uint,
) model.Todo {
	t.Helper()
	startsat := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	content := "This is a test todo item"
	item, err := service.Create(context.Background(), todo.CreateCommand{
		GroupID:    groupID,
		CreatorID:  creatorID,
		Title:      "Test Todo",
		Content:    &content,
		StartsAt:   &startsat,
		RepeatMode: model.RepeatOnce,
	})
	if err != nil {
		t.Fatalf("Failed to create todo: %v", err)
	}

	return item
}

var (
	// groupID,user1,user2,...
	comb = [][]uint{
		{1, 1, 2, 3, 4, 5},
		{2, 6, 7, 8, 9, 10},
		{3, 11, 12, 13, 14, 15},
	}
)

func TestTodoPatchRoles(t *testing.T) {
	// 1. 创建临时数据库，再按 comb 写入用户、群组和群成员。
	db := authTestDB(t)
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	for _, row := range comb {
		groupID := row[0]
		userIDs := row[1:]

		for _, userID := range userIDs {
			user := model.User{
				ID: userID, Username: fmt.Sprintf("user%d", userID),
				Suffix: 12345, Email: fmt.Sprintf("user%d@example.com", userID),
				PasswordHash: "test-hash", Nickname: fmt.Sprintf("用户%d", userID),
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
		}

		group := model.Group{
			ID: groupID, Name: fmt.Sprintf("群%d", groupID), Code: fmt.Sprintf("%06d", groupID),
			OwnerID: userIDs[0],
		}
		if err := db.Create(&group).Error; err != nil {
			t.Fatal(err)
		}
		for _, userID := range userIDs {
			member := model.GroupMember{GroupID: groupID, UserID: userID}
			if err := db.Create(&member).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	service := todo.NewService(todo.NewGORMRepository(db))
	ctx := context.Background()
	// 群1的群主是用户1，但这条 Todo 的创建者是用户2。
	item := createTestTodo(t, service, 1, 2)
	other := createTestTodo(t, service, 2, 7)

	// 在数据库里检查一条权限，避免只检查 err 却没有验证实际修改。
	checkRole := func(todoID, userID uint, want model.TodoRole) {
		t.Helper()
		var member model.TodoMember
		if err := db.Where("todo_id = ? AND user_id = ?", todoID, userID).
			First(&member).Error; err != nil {
			t.Fatal(err)
		}
		if member.Role != want {
			t.Fatalf("todo %d user %d: got %q, want %q", todoID, userID, member.Role, want)
		}
	}

	// 2. 批量升为 editor，再降回 viewer；每次重复提交，且名单中有重复 ID。
	for _, role := range []model.TodoRole{model.TodoEditor, model.TodoViewer} {
		for attempt := 0; attempt < 2; attempt++ {
			if err := service.PatchRole(ctx, 2, item.ID, []uint{3, 4, 3}, role); err != nil {
				t.Fatal(err)
			}
			checkRole(item.ID, 3, role)
			checkRole(item.ID, 4, role)
			checkRole(item.ID, 2, model.TodoEditor)
			checkRole(item.ID, 5, model.TodoViewer)
			checkRole(other.ID, 7, model.TodoEditor)
			checkRole(other.ID, 8, model.TodoViewer)
		}
	}

	// 3. 用户6属于群2：混进目标名单后，整批失败，用户3也不能被修改。
	err := service.PatchRole(ctx, 2, item.ID, []uint{3, 6}, model.TodoEditor)
	if !errors.Is(err, apperrors.ErrTodoMemberNotFound) {
		t.Fatalf("got %v, want ErrTodoMemberNotFound", err)
	}
	checkRole(item.ID, 3, model.TodoViewer)

	// 4. 群主即使是 editor，也不能替创建者管理权限。
	if err := service.PatchRole(ctx, 2, item.ID, []uint{1}, model.TodoEditor); err != nil {
		t.Fatal(err)
	}
	err = service.PatchRole(ctx, 1, item.ID, []uint{3}, model.TodoEditor)
	if !errors.Is(err, apperrors.ErrTodoPermissionDenied) {
		t.Fatalf("got %v, want ErrTodoPermissionDenied", err)
	}
	checkRole(item.ID, 3, model.TodoViewer)

	// 5. 不能把创建者降为 viewer。
	err = service.PatchRole(ctx, 2, item.ID, []uint{2}, model.TodoViewer)
	if !errors.Is(err, apperrors.ErrTodoRoleInvalid) {
		t.Fatalf("got %v, want ErrTodoRoleInvalid", err)
	}
	checkRole(item.ID, 2, model.TodoEditor)

	// 6. 模拟更新用户4时数据库出错：用户3和用户4的修改都必须回滚。
	if err := db.Exec(fmt.Sprintf(
		"CREATE TEMP TRIGGER reject_role_update AFTER UPDATE OF role ON todo_members "+
			"WHEN NEW.todo_id = %d AND NEW.user_id = 4 "+
			"BEGIN SELECT RAISE(FAIL, 'test role update failure'); END", item.ID,
	)).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.PatchRole(ctx, 2, item.ID, []uint{3, 4}, model.TodoEditor); err == nil {
		t.Fatal("expected database error")
	}
	checkRole(item.ID, 3, model.TodoViewer)
	checkRole(item.ID, 4, model.TodoViewer)

	// 7. 创建者退群后，留下的 TodoMember 也不能让他继续管理权限。
	if err := db.Where("group_id = ? AND user_id = ?", 1, 2).
		Delete(&model.GroupMember{}).Error; err != nil {
		t.Fatal(err)
	}
	err = service.PatchRole(ctx, 2, item.ID, []uint{3}, model.TodoEditor)
	if !errors.Is(err, apperrors.ErrGroupAccessDenied) {
		t.Fatalf("got %v, want ErrGroupAccessDenied", err)
	}
	checkRole(item.ID, 3, model.TodoViewer)
}
