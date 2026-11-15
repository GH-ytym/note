package internal

import (
	"context"
	"encoding/json"
	"note/internal/model"
	"note/internal/notification"
	"testing"
)

func TestNotificationBatchRead(t *testing.T) {
	db, owner, member, group := groupSchemaFixture(t)
	notices := make([]model.Notification, 502)
	for i := range notices {
		notices[i] = model.Notification{ReceiverID: owner.ID, ActorID: member.ID, GroupID: group.ID, Type: model.Joined}
	}
	if err := db.CreateInBatches(&notices, 100).Error; err != nil {
		t.Fatal(err)
	}
	foreign := model.Notification{ReceiverID: member.ID, ActorID: owner.ID, GroupID: group.ID, Type: model.Joined}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateNotificationStateSchema(db); err != nil {
		t.Fatal(err)
	}
	ids := make([]uint, 0, 504)
	for _, n := range notices[:501] {
		ids = append(ids, n.ID)
	}
	ids = append(ids, notices[0].ID, foreign.ID, 999999)
	repo := notification.NewGORMRepository(db)
	// 初始批次横跨 500 个参数边界，最后一条自身通知未选中。
	if err := repo.MarkRead(context.Background(), owner.ID, ids); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkRead(context.Background(), owner.ID, ids); err != nil {
		t.Fatal(err)
	}
	var state model.NotificationState
	if err := db.Where("user_id = ?", owner.ID).First(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state.Revision != 1 || state.UnreadCount != 1 {
		t.Fatalf("state: %+v", state)
	}
	var tasks []model.Outbox
	if err := db.Where("receiver_id = ?", owner.ID).Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks: %d", len(tasks))
	}
	var change notification.Change
	if err := json.Unmarshal([]byte(tasks[0].Data), &change); err != nil {
		t.Fatal(err)
	}
	if len(change.ReadIDs) != 501 || len(change.Upserts) != 0 || change.ReadAt == nil {
		t.Fatalf("bad change: %+v", change)
	}
	var unread int64
	if err := db.Model(&model.Notification{}).Where("read_at IS NULL").Count(&unread).Error; err != nil {
		t.Fatal(err)
	}
	if unread != 2 {
		t.Fatalf("unread: %d", unread)
	}
	// 计数不合法导致入队失败时，read_at 必须一起回滚。
	if err := db.Model(&model.NotificationState{}).Where("user_id = ?", owner.ID).Update("unread_count", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkRead(context.Background(), owner.ID, []uint{notices[501].ID}); err == nil {
		t.Fatal("expected invalid count error")
	}
	var last model.Notification
	if err := db.First(&last, notices[501].ID).Error; err != nil {
		t.Fatal(err)
	}
	if last.ReadAt != nil {
		t.Fatal("failed transaction did not roll back")
	}
}
