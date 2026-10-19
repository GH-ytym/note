package internal

import (
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
