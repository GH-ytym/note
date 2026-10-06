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
	"note/internal/group"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/router"
	"note/internal/todo"

	"gorm.io/gorm"
)

func groupManagementFixture(t *testing.T) (*gorm.DB, model.User, model.User, model.User, model.Group, model.Group) {
	t.Helper()
	db, owner, member, first := groupSchemaFixture(t)
	outsider := model.User{
		Username: "outsider", Suffix: 12345, Nickname: "其他群成员",
		Email: "outsider@example.com", PasswordHash: "test-hash",
	}
	if err := db.Create(&outsider).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&member).Update("avatar", "member-avatar").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&first).Update("code", "ABC123").Error; err != nil {
		t.Fatal(err)
	}
	second := model.Group{Name: "保留的群", OwnerID: owner.ID, Code: "Q7W8E9"}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	joinedAt := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	members := []model.GroupMember{
		{GroupID: first.ID, UserID: owner.ID, JoinedAt: joinedAt.Add(time.Hour)},
		{GroupID: first.ID, UserID: member.ID, JoinedAt: joinedAt},
		{GroupID: second.ID, UserID: owner.ID, JoinedAt: joinedAt},
		{GroupID: second.ID, UserID: outsider.ID, JoinedAt: joinedAt},
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatal(err)
	}
	return db, owner, member, outsider, first, second
}

func groupManagementRequest(t *testing.T, db *gorm.DB) func(string, string, uint, string) *httptest.ResponseRecorder {
	t.Helper()
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	gh := handler.NewGroupHandler(group.NewService(group.NewGORMRepository(db)))
	r := router.NewWithWeb(nil, nil, nil, nil, nil, gh, nil, tokens, "")
	return func(method, path string, userID uint, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
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
}

func TestGroupMembersHTTPVisibilityAndProfiles(t *testing.T) {
	db, owner, member, outsider, first, _ := groupManagementFixture(t)
	request := groupManagementRequest(t, db)
	for _, tc := range []struct {
		userID uint
		prefix string
	}{{owner.ID, ""}, {member.ID, "/api"}} {
		res := request(http.MethodGet, fmt.Sprintf("%s/groups/%d/members", tc.prefix, first.ID), tc.userID, "")
		if res.Code != http.StatusOK {
			t.Fatalf("list members: %d %s", res.Code, res.Body.String())
		}
		var members []handler.GroupMemberResponse
		if err := json.Unmarshal(res.Body.Bytes(), &members); err != nil {
			t.Fatal(err)
		}
		if len(members) != 2 || members[0].ID != member.ID || members[1].ID != owner.ID {
			t.Fatalf("incorrect group or joined_at ordering: %+v", members)
		}
		if members[0].Username != member.Username || members[0].Suffix != member.Suffix ||
			members[0].Nickname != member.Nickname || members[0].Avatar != "member-avatar" || members[0].JoinedAt.IsZero() {
			t.Fatalf("incorrect public profile: %+v", members[0])
		}
		var raw []map[string]json.RawMessage
		if err := json.Unmarshal(res.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		for _, profile := range raw {
			if len(profile) != 6 {
				t.Fatalf("unexpected profile fields: %+v", profile)
			}
			for _, field := range []string{"id", "username", "suffix", "nickname", "avatar", "joined_at"} {
				if _, ok := profile[field]; !ok {
					t.Fatalf("missing public field %s", field)
				}
			}
		}
	}
	// DTO 限制输出之外，数据库查询也不加载账号私密资料。
	loaded, err := group.NewGORMRepository(db).ListMembers(context.Background(), first.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, membership := range loaded {
		if membership.User == nil || membership.User.Email != "" || membership.User.PasswordHash != "" {
			t.Fatalf("member preload includes private account data: %+v", membership.User)
		}
	}
	for _, tc := range []struct {
		path   string
		userID uint
		status int
	}{
		{fmt.Sprintf("/api/groups/%d/members", first.ID), 0, http.StatusUnauthorized},
		{fmt.Sprintf("/api/groups/%d/members?user_id=%d", first.ID, owner.ID), outsider.ID, http.StatusForbidden},
		{"/api/groups/999999/members", owner.ID, http.StatusNotFound},
		{"/api/groups/abc/members", owner.ID, http.StatusBadRequest},
	} {
		res := request(http.MethodGet, tc.path, tc.userID, "")
		if res.Code != tc.status {
			t.Fatalf("%s: got %d, want %d: %s", tc.path, res.Code, tc.status, res.Body.String())
		}
	}
	if err := group.NewGORMRepository(db).Quit(context.Background(), first.ID, member.ID, nil); err != nil {
		t.Fatal(err)
	}
	res := request(http.MethodGet, fmt.Sprintf("/api/groups/%d/members", first.ID), member.ID, "")
	if res.Code != http.StatusForbidden {
		t.Fatalf("former member can read membership: %d %s", res.Code, res.Body.String())
	}
}

func seedGroupManagementTodo(t *testing.T, db *gorm.DB, groupID, creatorID, completedUserID uint) model.Todo {
	t.Helper()
	date := time.Date(2026, 10, 3, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	startsAt := date.Add(9 * time.Hour)
	service := todo.NewService(todo.NewGORMRepository(db))
	item, err := service.Create(context.Background(), todo.CreateCommand{
		GroupID: groupID, CreatorID: creatorID, Title: "群内协作记录", StartsAt: &startsAt,
		RepeatMode: model.RepeatCustom, CustomDates: []time.Time{date},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetOccurrenceDone(context.Background(), item.ID, completedUserID, date, true); err != nil {
		t.Fatal(err)
	}
	return item
}

func TestGroupDismissHTTPPermissionsAndCleanup(t *testing.T) {
	db, owner, member, outsider, first, second := groupManagementFixture(t)
	seedGroupManagementTodo(t, db, first.ID, owner.ID, member.ID)
	retained := seedGroupManagementTodo(t, db, second.ID, owner.ID, outsider.ID)
	request := groupManagementRequest(t, db)
	path := fmt.Sprintf("/api/groups/%d/dismiss", first.ID)
	for _, tc := range []struct {
		path   string
		userID uint
		status int
	}{
		{path, 0, http.StatusUnauthorized},
		{path, member.ID, http.StatusForbidden},
		{path, outsider.ID, http.StatusForbidden},
		{"/api/groups/999999/dismiss", owner.ID, http.StatusNotFound},
		{"/api/groups/abc/dismiss", owner.ID, http.StatusBadRequest},
	} {
		// 请求体里的身份不能覆盖 JWT。
		res := request(http.MethodPost, tc.path, tc.userID, fmt.Sprintf(`{"user_id":%d,"owner_id":%d}`, owner.ID, owner.ID))
		if res.Code != tc.status {
			t.Fatalf("dismiss user %d: got %d, want %d: %s", tc.userID, res.Code, tc.status, res.Body.String())
		}
	}
	res := request(http.MethodPost, path, owner.ID, "")
	if res.Code != http.StatusNoContent || res.Body.Len() != 0 {
		t.Fatalf("owner dismiss: %d %s", res.Code, res.Body.String())
	}
	for _, tc := range []struct {
		model any
		count int64
	}{
		{&model.Group{}, 1}, {&model.GroupMember{}, 2}, {&model.Todo{}, 1},
		{&model.TodoDate{}, 1}, {&model.TodoMember{}, 2}, {&model.TodoCompletion{}, 1},
		{&model.User{}, 3},
	} {
		var count int64
		if err := db.Model(tc.model).Count(&count).Error; err != nil || count != tc.count {
			t.Fatalf("cleanup changed %T unexpectedly: count=%d want=%d err=%v", tc.model, count, tc.count, err)
		}
	}
	var retainedTodo model.Todo
	if err := db.Preload("CustomDates").Preload("Members").First(&retainedTodo, retained.ID).Error; err != nil ||
		retainedTodo.GroupID != second.ID || retainedTodo.CreatorID != owner.ID ||
		len(retainedTodo.CustomDates) != 1 || len(retainedTodo.Members) != 2 {
		t.Fatalf("other group's todo changed: %+v err=%v", retainedTodo, err)
	}
	var completion model.TodoCompletion
	if err := db.Where("todo_id = ?", retained.ID).First(&completion).Error; err != nil ||
		len(completion.Records) != 1 || completion.Records[0].UserID != outsider.ID {
		t.Fatalf("other group's completion changed: %+v err=%v", completion, err)
	}
	res = request(http.MethodGet, fmt.Sprintf("/api/groups/%d/members", first.ID), owner.ID, "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("dismissed group still readable: %d %s", res.Code, res.Body.String())
	}
}

func TestGroupDismissRequiresCurrentOwner(t *testing.T) {
	db, owner, member, _, first, _ := groupManagementFixture(t)
	repo := group.NewGORMRepository(db)
	if err := repo.Quit(context.Background(), first.ID, owner.ID, &member.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Join(context.Background(), first.ID, owner.ID, "ABC123"); err != nil {
		t.Fatal(err)
	}
	// 旧群主即使重新加入，也不能凭旧身份解散。
	if err := repo.Dismiss(context.Background(), first.ID, owner.ID); !errors.Is(err, apperrors.ErrGroupDismissDenied) {
		t.Fatalf("former owner dismiss: %v", err)
	}
	// 当前 OwnerID 也不能代替当前群成员资格。
	if err := db.Where("group_id = ? AND user_id = ?", first.ID, member.ID).
		Delete(&model.GroupMember{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.Dismiss(context.Background(), first.ID, member.ID); !errors.Is(err, apperrors.ErrGroupAccessDenied) {
		t.Fatalf("owner without membership dismiss: %v", err)
	}
}

func TestGroupDismissRollsBackAllData(t *testing.T) {
	db, owner, member, _, first, _ := groupManagementFixture(t)
	item := seedGroupManagementTodo(t, db, first.ID, owner.ID, member.ID)
	var before model.TodoCompletion
	if err := db.Where("todo_id = ?", item.ID).First(&before).Error; err != nil {
		t.Fatal(err)
	}
	// 最后一步故意失败，前面已删除的内容和成员关系必须全部恢复。
	trigger := fmt.Sprintf(`CREATE TEMP TRIGGER reject_group_dismiss
		BEFORE DELETE ON groups WHEN OLD.id = %d
		BEGIN SELECT RAISE(ABORT, 'test dismissal failure'); END`, first.ID)
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	request := groupManagementRequest(t, db)
	res := request(http.MethodPost, fmt.Sprintf("/groups/%d/dismiss", first.ID), owner.ID, "")
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("dismiss should fail: %d %s", res.Code, res.Body.String())
	}
	for _, tc := range []struct {
		model any
		count int64
	}{
		{&model.Group{}, 2}, {&model.GroupMember{}, 4}, {&model.Todo{}, 1},
		{&model.TodoDate{}, 1}, {&model.TodoMember{}, 2}, {&model.TodoCompletion{}, 1},
		{&model.User{}, 3},
	} {
		var count int64
		if err := db.Model(tc.model).Count(&count).Error; err != nil || count != tc.count {
			t.Fatalf("dismiss rollback lost %T: count=%d want=%d err=%v", tc.model, count, tc.count, err)
		}
	}
	var after model.TodoCompletion
	if err := db.First(&after, before.ID).Error; err != nil || len(after.Records) != 1 ||
		after.Records[0].UserID != before.Records[0].UserID ||
		!after.Records[0].CompletedAt.Equal(before.Records[0].CompletedAt) {
		t.Fatalf("rollback changed completion history: %+v err=%v", after, err)
	}
}
