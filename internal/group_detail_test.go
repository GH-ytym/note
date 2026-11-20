package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"note/internal/group"
	"note/internal/model"
	"note/internal/notification"
	"strings"
	"testing"
)

func TestNotificationDetailFreshAndPrivate(t *testing.T) {
	db, owner, user, g := groupSchemaFixture(t)
	initializeJoinNotificationStates(t, db, owner, user)
	if err := db.Model(&g).Updates(map[string]any{"code": "ABC123", "policy": model.Approval}).Error; err != nil {
		t.Fatal(err)
	}
	repo := group.NewGORMRepository(db)
	req, msg, err := repo.Join(context.Background(), user.ID, "ABC123")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user).Updates(map[string]any{"avatar": "https://example.com/new.jpg", "nickname": "新昵称"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&g).Updates(map[string]any{"name": "新群名"}).Error; err != nil {
		t.Fatal(err)
	}
	request := notificationRequest(t, db)
	path := fmt.Sprintf("/api/notifications/%d", msg.ID)
	for _, tc := range []struct {
		uid  uint
		want int
	}{{0, 401}, {user.ID, 404}, {owner.ID, 200}} {
		res := request("GET", path, tc.uid, "")
		if res.Code != tc.want {
			t.Fatalf("status %d: %s", res.Code, res.Body.String())
		}
	}
	if err := repo.Review(context.Background(), owner.ID, req.ID, model.Rejected); err != nil {
		t.Fatal(err)
	}
	res := request("GET", path, owner.ID, "")
	var detail notification.Detail
	if err := json.Unmarshal(res.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Card.ActorName != "新昵称" || detail.Card.ActorAvatar != "https://example.com/new.jpg" || detail.Card.RequestStatus != model.Rejected || detail.Group.Name != "新群名" {
		t.Fatalf("stale detail: %+v", detail)
	}
	for _, field := range []string{"password", "email", "members", "code", "owner_id"} {
		if strings.Contains(res.Body.String(), `"`+field+`"`) {
			t.Fatalf("private field %s leaked", field)
		}
	}
}

func TestGroupAvatarUploadDisabled(t *testing.T) {
	db, owner, _, g := groupSchemaFixture(t)
	request := notificationRequest(t, db)
	for _, method := range []string{"PUT", "DELETE"} {
		res := request(method, fmt.Sprintf("/api/groups/%d/avatar", g.ID), owner.ID, "")
		if res.Code != 404 {
			t.Fatalf("group avatar endpoint remains enabled: %d", res.Code)
		}
	}
}
