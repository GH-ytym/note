package internal

import (
	"fmt"
	"net/http"
	"note/internal/model"
	"testing"
)

func TestNotificationMarkReadHTTP(t *testing.T) {
	db, owner, member, group := groupSchemaFixture(t)
	notices := []model.Notification{
		{ReceiverID: owner.ID, ActorID: member.ID, GroupID: group.ID, Type: model.Joined},
		{ReceiverID: member.ID, ActorID: owner.ID, GroupID: group.ID, Type: model.Joined},
	}
	if err := db.Create(&notices).Error; err != nil {
		t.Fatal(err)
	}
	request := notificationRequest(t, db)
	path := "/api/notifications/read"
	if res := request(http.MethodPost, path, 0, `{"ids":[1]}`); res.Code != 401 {
		t.Fatal(res.Code)
	}
	for _, body := range []string{`{}`, `{"ids":[]}`, `{"ids":null}`, `{"ids":[0]}`, `{"ids":[-1]}`, `{"ids":[1.5]}`, `{"ids":"1"}`, `{`} {
		if res := request(http.MethodPost, path, owner.ID, body); res.Code != 400 {
			t.Fatalf("%s: %d %s", body, res.Code, res.Body.String())
		}
	}
	body := fmt.Sprintf(`{"ids":[%d,%d,%d],"user_id":%d}`, notices[0].ID, notices[0].ID, notices[1].ID, member.ID)
	for i := 0; i < 2; i++ {
		if res := request(http.MethodPost, path, owner.ID, body); res.Code != 204 || res.Body.Len() != 0 {
			t.Fatalf("read: %d %s", res.Code, res.Body.String())
		}
	}
	var own, other model.Notification
	if err := db.First(&own, notices[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&other, notices[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if own.ReadAt == nil || other.ReadAt != nil {
		t.Fatal("ownership/read state incorrect")
	}
	var state model.NotificationState
	if err := db.Where("user_id = ?", owner.ID).First(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state.Revision != 1 || state.UnreadCount != 0 {
		t.Fatalf("state: %+v", state)
	}
	var count int64
	if err := db.Model(&model.Outbox{}).Where("receiver_id = ?", owner.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("tasks: %d", count)
	}
}
