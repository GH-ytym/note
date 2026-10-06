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
	"note/internal/group"
	"note/internal/handler"
	"note/internal/model"
	"note/internal/notification"
	"note/internal/router"

	"gorm.io/gorm"
)

func notificationRequest(t *testing.T, db *gorm.DB) func(string, string, uint, string) *httptest.ResponseRecorder {
	t.Helper()
	tokens, err := auth.NewTokenManager("test-only-secret-0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	gh := handler.NewGroupHandler(group.NewService(group.NewGORMRepository(db)))
	hub := notification.NewHub()
	t.Cleanup(hub.Close)
	nh := handler.NewNotificationHandler(notification.NewService(notification.NewGORMRepository(db)), hub)
	r := router.NewWithWeb(nil, nil, nil, nil, nil, gh, nh, tokens, "")
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

func TestGroupJoinNotificationList(t *testing.T) {
	for _, tc := range []struct {
		policy     model.GroupPolicy
		joinStatus int
		noticeType model.NotificationType
	}{
		{model.Public, http.StatusOK, model.Joined},
		{model.Approval, http.StatusAccepted, model.JoinRequested},
	} {
		t.Run(string(tc.policy), func(t *testing.T) {
			db, owner, member, item := groupSchemaFixture(t)
			if err := db.Model(&item).Updates(map[string]any{
				"code": "ABC123", "policy": tc.policy,
			}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.GroupMember{GroupID: item.ID, UserID: owner.ID}).Error; err != nil {
				t.Fatal(err)
			}
			member.Nickname = ""
			member.Avatar = "https://example.com/avatar.png"
			if err := db.Model(&member).Select("nickname", "avatar").Updates(&member).Error; err != nil {
				t.Fatal(err)
			}
			request := notificationRequest(t, db)
			joined := request(http.MethodPost, fmt.Sprintf("/api/groups/%d/join", item.ID), member.ID, `{"code":"ABC123"}`)
			if joined.Code != tc.joinStatus {
				t.Fatalf("join: got %d, want %d: %s", joined.Code, tc.joinStatus, joined.Body.String())
			}

			for _, prefix := range []string{"", "/api"} {
				res := request(http.MethodGet, prefix+"/notifications", owner.ID, "")
				if res.Code != http.StatusOK {
					t.Fatalf("list: got %d: %s", res.Code, res.Body.String())
				}
				if res.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("notification response allows browser caching")
				}
				var inbox notification.Inbox
				if err := json.Unmarshal(res.Body.Bytes(), &inbox); err != nil {
					t.Fatal(err)
				}
				if len(inbox.Items) != 1 || inbox.UnreadCount != 1 {
					t.Fatalf("unexpected owner inbox: %+v", inbox)
				}
				card := inbox.Items[0]
				if card.Type != tc.noticeType || card.ActorID != member.ID ||
					card.ActorName != fmt.Sprintf("%s#%05d", member.Username, member.Suffix) ||
					card.ActorAvatar != member.Avatar || card.GroupID != item.ID || card.GroupName != item.Name ||
					card.CreatedAt.IsZero() || card.ReadAt != nil {
					t.Fatalf("unexpected notification card: %+v", card)
				}
				if tc.policy == model.Public {
					if card.Request != nil || !strings.Contains(res.Body.String(), `"request":null`) {
						t.Fatal("public join unexpectedly contains an application")
					}
				} else if card.Request == nil || card.Request.ID == 0 ||
					card.Request.Kind != model.Application || card.Request.Status != model.Pending ||
					card.Request.SenderID != member.ID || card.Request.ReceiverID != owner.ID ||
					card.Request.CreatedAt.IsZero() || card.Request.HandledAt != nil {
					t.Fatalf("missing or invalid application card: %+v", card.Request)
				}

				// 用户身份来自 JWT，请求参数不能切换到另一个人的通知列表。
				res = request(http.MethodGet, fmt.Sprintf("%s/notifications?receiver_id=%d&user_id=%d", prefix, owner.ID, owner.ID), member.ID, "")
				if res.Code != http.StatusOK {
					t.Fatalf("member inbox: got %d: %s", res.Code, res.Body.String())
				}
				var empty notification.Inbox
				if err := json.Unmarshal(res.Body.Bytes(), &empty); err != nil {
					t.Fatal(err)
				}
				if empty.Items == nil || len(empty.Items) != 0 || empty.UnreadCount != 0 {
					t.Fatalf("member can see owner's notices or gets null items: %+v", empty)
				}
			}
		})
	}
}

func TestNotificationListLimitAndUnreadCount(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	now := time.Now().UTC()
	notices := make([]model.Notification, 0, 52)
	for i := 0; i < 51; i++ {
		notices = append(notices, model.Notification{
			ReceiverID: owner.ID, ActorID: member.ID, GroupID: item.ID, Type: model.Joined,
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	notices = append(notices, model.Notification{
		ReceiverID: member.ID, ActorID: owner.ID, GroupID: item.ID, Type: model.Joined,
		ReadAt: &now, CreatedAt: now,
	})
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&notices).Error; err != nil {
		t.Fatal(err)
	}
	request := notificationRequest(t, db)
	res := request(http.MethodGet, "/api/notifications", owner.ID, "")
	if res.Code != http.StatusOK {
		t.Fatalf("list: got %d: %s", res.Code, res.Body.String())
	}
	var inbox notification.Inbox
	if err := json.Unmarshal(res.Body.Bytes(), &inbox); err != nil {
		t.Fatal(err)
	}
	if len(inbox.Items) != 50 || inbox.UnreadCount != 51 {
		t.Fatalf("list limit incorrectly limits unread count: items=%d unread=%d", len(inbox.Items), inbox.UnreadCount)
	}
	for i, card := range inbox.Items {
		if card.ID != notices[50-i].ID {
			t.Fatalf("card %d: got ID %d, want %d", i, card.ID, notices[50-i].ID)
		}
	}
	res = request(http.MethodGet, "/api/notifications", member.ID, "")
	if res.Code != http.StatusOK {
		t.Fatalf("member inbox: got %d: %s", res.Code, res.Body.String())
	}
	var memberInbox notification.Inbox
	if err := json.Unmarshal(res.Body.Bytes(), &memberInbox); err != nil {
		t.Fatal(err)
	}
	if len(memberInbox.Items) != 1 || memberInbox.Items[0].ID != notices[51].ID ||
		memberInbox.Items[0].ReadAt == nil || memberInbox.UnreadCount != 0 {
		t.Fatalf("unexpected read notification: %+v", memberInbox)
	}
}

func TestNotificationRepositoryRecipientBoundary(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	notice := model.Notification{
		ReceiverID: owner.ID, ActorID: member.ID, GroupID: item.ID, Type: model.Joined,
	}
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&notice).Error; err != nil {
		t.Fatal(err)
	}
	repo := notification.NewGORMRepository(db)
	if _, err := repo.ByIDForReceiver(context.Background(), member.ID, notice.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("another receiver can query the notice: %v", err)
	}
	loaded, err := repo.ByIDForReceiver(context.Background(), owner.ID, notice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actor == nil || loaded.Group == nil || loaded.Receiver != nil ||
		loaded.Actor.ID != member.ID || loaded.Actor.Username != member.Username ||
		loaded.Actor.Email != "" || loaded.Actor.PasswordHash != "" || !loaded.Actor.CreatedAt.IsZero() ||
		loaded.Group.ID != item.ID || loaded.Group.Name != item.Name || loaded.Group.OwnerID != 0 {
		t.Fatalf("unexpected projected associations: %+v", loaded)
	}
}
