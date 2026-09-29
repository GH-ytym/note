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
)

func TestGroupCreationHTTP(t *testing.T) {
	db, firstUser, secondUser, _ := groupSchemaFixture(t)
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	gh := handler.NewGroupHandler(group.NewService(group.NewGORMRepository(db)))
	r := router.NewWithWeb(nil, nil, nil, nil, nil, gh, tokens, "")
	// 两个不同用户分别使用两套路由；请求体伪造 owner_id 不得改变真实群主。
	for i, user := range []model.User{firstUser, secondUser} {
		path := []string{"/groups", "/api/groups"}[i]
		token, err := tokens.Generate(user.ID)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"name":"  Go 学习群  ","owner_id":999999,"members":[{"user_id":999999}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusCreated {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
		var result handler.GroupResponse
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.ID == 0 || result.OwnerID != user.ID || result.Name != "Go 学习群" || result.CreatedAt.IsZero() {
			t.Fatalf("unexpected response: %+v", result)
		}
		var saved model.Group
		if err := db.Preload("Members").First(&saved, result.ID).Error; err != nil {
			t.Fatal(err)
		}
		if saved.OwnerID != user.ID || len(saved.Members) != 1 || saved.Members[0].UserID != user.ID || saved.Members[0].JoinedAt.IsZero() {
			t.Fatalf("group and owner membership disagree: %+v", saved)
		}
	}

	validToken, err := tokens.Generate(firstUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	missingUserToken, err := tokens.Generate(999999)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body, token string
		status            int
	}{
		{"unauthenticated", `{"name":"test"}`, "", http.StatusUnauthorized},
		{"invalid token", `{"name":"test"}`, "invalid", http.StatusUnauthorized},
		{"missing user", `{"name":"test"}`, missingUserToken, http.StatusUnauthorized},
		{"invalid JSON", "{", validToken, http.StatusBadRequest},
		{"missing name", "{}", validToken, http.StatusBadRequest},
		{"blank name", `{"name":"   "}`, validToken, http.StatusBadRequest},
		{"long name", fmt.Sprintf(`{"name":%q}`, strings.Repeat("群", 81)), validToken, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before int64
			if err := db.Model(&model.Group{}).Count(&before).Error; err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/groups", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			res := httptest.NewRecorder()
			r.ServeHTTP(res, req)
			if res.Code != tc.status {
				t.Fatalf("got %d, want %d: %s", res.Code, tc.status, res.Body.String())
			}
			var after int64
			if err := db.Model(&model.Group{}).Count(&after).Error; err != nil || after != before {
				t.Fatalf("failed request created a group: before=%d after=%d err=%v", before, after, err)
			}
		})
	}
}

func TestGroupCreationRollsBackOnMembershipFailure(t *testing.T) {
	db, owner, _, existing := groupSchemaFixture(t)
	// 仅在临时测试库中让第二次写入失败，验证已经成功的第一次写入确实回滚。
	if err := db.Exec(`CREATE TRIGGER reject_group_membership
		BEFORE INSERT ON group_members BEGIN
		SELECT RAISE(ABORT, 'test membership failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	s := group.NewService(group.NewGORMRepository(db))
	_, err := s.Create(context.Background(), owner.ID, "应被回滚的群")
	if err == nil || !strings.Contains(err.Error(), "test membership failure") {
		t.Fatalf("expected membership failure, got %v", err)
	}
	var groups []model.Group
	if err := db.Find(&groups).Error; err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].ID != existing.ID {
		t.Fatalf("failed transaction changed groups: %+v", groups)
	}
	var count int64
	if err := db.Model(&model.GroupMember{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed transaction left members: count=%d err=%v", count, err)
	}
}

func TestGroupCreationReturnsInitialMembers(t *testing.T) {
	db, owner, _, _ := groupSchemaFixture(t)
	s := group.NewService(group.NewGORMRepository(db))
	item, err := s.Create(context.Background(), owner.ID, "初始成员")
	if err != nil {
		t.Fatal(err)
	}
	// 不执行 Preload，Service 返回的对象本身就应带有完整的初始成员关系。
	if item.ID == 0 || len(item.Members) != 1 {
		t.Fatalf("missing initial membership: %+v", item)
	}
	member := item.Members[0]
	if member.GroupID != item.ID || member.UserID != owner.ID || member.JoinedAt.IsZero() {
		t.Fatalf("association was not populated: %+v", member)
	}
	var saved model.GroupMember
	if err := db.Where("group_id = ? AND user_id = ?", item.ID, owner.ID).First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if !saved.JoinedAt.Equal(member.JoinedAt) {
		t.Fatal("returned membership differs from stored membership")
	}
}

func TestGroupCreationServiceValidation(t *testing.T) {
	// 无有效参数时不应触及 Repository，nil 在这里用于检查这个边界。
	s := group.NewService(nil)
	for _, tc := range []struct {
		id   uint
		name string
		want error
	}{
		{0, "学习群", apperrors.ErrGroupUnauthenticated},
		{1, " \t\n", apperrors.ErrGroupNameInvalid},
		{1, strings.Repeat("群", 81), apperrors.ErrGroupNameInvalid},
	} {
		if _, err := s.Create(context.Background(), tc.id, tc.name); !errors.Is(err, tc.want) {
			t.Fatalf("got %v, want %v", err, tc.want)
		}
	}
}
