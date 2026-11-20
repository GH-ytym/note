package internal

import (
	"strings"
	"testing"
	"time"

	"note/internal/model"
)

func TestNotificationGroupCascadeMigration(t *testing.T) {
	db, owner, actor, group := groupSchemaFixture(t)
	if err := db.Migrator().DropTable(&model.Notification{}); err != nil {
		t.Fatal(err)
	}
	// 重现旧表：群删除会级联删除通知。
	if err := db.Exec(`CREATE TABLE notifications (
		id integer PRIMARY KEY AUTOINCREMENT,
		receiver_id integer NOT NULL, actor_id integer NOT NULL,
		group_id integer NOT NULL, join_request_id integer,
		type text NOT NULL, read_at datetime, created_at datetime,
		CONSTRAINT fk_notifications_receiver FOREIGN KEY(receiver_id) REFERENCES users(id) ON DELETE CASCADE,
		CONSTRAINT fk_notifications_actor FOREIGN KEY(actor_id) REFERENCES users(id) ON DELETE RESTRICT,
		CONSTRAINT fk_notifications_group FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE,
		CONSTRAINT fk_notifications_join_request FOREIGN KEY(join_request_id) REFERENCES group_join_requests(id) ON DELETE SET NULL,
		CONSTRAINT ck_notifications_type CHECK(type IN ('joined','join_requested'))
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE INDEX idx_notifications_receiver_read ON notifications(receiver_id, read_at)").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	n := model.Notification{ReceiverID: owner.ID, ActorID: actor.ID, GroupID: group.ID,
		Type: model.Joined, ReadAt: &now, CreatedAt: now}
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&n).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateNotificationSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	var saved model.Notification
	if err := db.First(&saved, n.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.GroupID != group.ID || saved.ReceiverID != owner.ID || saved.ActorID != actor.ID ||
		saved.Type != n.Type || !saved.CreatedAt.Equal(now) || saved.ReadAt == nil || !saved.ReadAt.Equal(now) {
		t.Fatalf("migration changed notification: %+v", saved)
	}
	if !db.Migrator().HasIndex(&model.Notification{}, "idx_notifications_receiver_read") {
		t.Fatal("migration removed notification index")
	}
	if err := db.Delete(&group).Error; err == nil {
		t.Fatal("migrated group foreign key still allows deletion")
	}
	if err := db.First(&saved, n.ID).Error; err != nil {
		t.Fatalf("notification lost after rejected group deletion: %v", err)
	}
}

func TestNotificationReviewTypeMigration(t *testing.T) {
	db, owner, actor, g := groupSchemaFixture(t)
	var schema string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE name = 'notifications'").Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.Notification{}); err != nil {
		t.Fatal(err)
	}
	// 旧库外键已经是 RESTRICT，但通知类型约束尚未升级。
	if err := db.Exec(strings.ReplaceAll(schema, ",'join_reviewed'", "")).Error; err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"CREATE INDEX custom_notice_idx ON notifications(receiver_id, id)",
		"CREATE TABLE notice_audit (id integer)",
		"CREATE TRIGGER notice_audit_insert AFTER INSERT ON notifications BEGIN INSERT INTO notice_audit VALUES (NEW.id); END",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	msg := model.Notification{ReceiverID: owner.ID, ActorID: actor.ID, GroupID: g.ID, Type: model.Joined, ReadAt: &now, CreatedAt: now}
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&msg).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateDatabase(db); err != nil {
			t.Fatal(err)
		}
	}
	var saved model.Notification
	if err := db.First(&saved, msg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ReceiverID != msg.ReceiverID || saved.ActorID != msg.ActorID || saved.GroupID != msg.GroupID || saved.Type != msg.Type || saved.ReadAt == nil || !saved.ReadAt.Equal(now) || !saved.CreatedAt.Equal(now) {
		t.Fatalf("lost data: %+v", saved)
	}
	if !db.Migrator().HasIndex(&model.Notification{}, "custom_notice_idx") {
		t.Fatal("lost custom index")
	}
	result := model.Notification{ReceiverID: actor.ID, ActorID: owner.ID, GroupID: g.ID, Type: model.JoinReviewed}
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.ID <= msg.ID {
		t.Fatal("notification ID reused")
	}
	var count int64
	if err := db.Table("notice_audit").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("trigger missing or migration fired it: %d", count)
	}
	if err := db.Model(&result).Update("type", "invalid").Error; err == nil {
		t.Fatal("type constraint missing")
	}
	var violations []struct{ Table string }
	if err := db.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("foreign key violations: %+v", violations)
	}
}
