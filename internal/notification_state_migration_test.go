package internal

import (
	"testing"
	"time"

	"note/internal/model"
)

func TestNotificationStateMigration(t *testing.T) {
	for _, schema := range []string{"missing_table", "revision_only", "current"} {
		t.Run(schema, func(t *testing.T) {
			db, owner, member, group := groupSchemaFixture(t)
			noNotice := model.User{Username: "empty", Suffix: 12345, Email: "empty@example.com", PasswordHash: "test"}
			if err := db.Create(&noNotice).Error; err != nil {
				t.Fatal(err)
			}
			if !db.Migrator().HasColumn(&model.NotificationState{}, "UnreadCount") {
				t.Fatal("startup did not create notification state schema")
			}
			now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
			// 超过面板的 50 条限制：未读数必须统计全部通知。
			notices := make([]model.Notification, 0, 57)
			for i := 0; i < 55; i++ {
				notices = append(notices, model.Notification{ReceiverID: owner.ID, ActorID: member.ID,
					GroupID: group.ID, Type: model.Joined, CreatedAt: now})
			}
			notices = append(notices,
				model.Notification{ReceiverID: owner.ID, ActorID: member.ID, GroupID: group.ID, Type: model.Joined, CreatedAt: now, ReadAt: &now},
				model.Notification{ReceiverID: member.ID, ActorID: owner.ID, GroupID: group.ID, Type: model.Joined, CreatedAt: now, ReadAt: &now})
			if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&notices).Error; err != nil {
				t.Fatal(err)
			}
			wantRevision := uint64(0)
			wantCount := int64(55)
			switch schema {
			case "missing_table", "revision_only":
				if err := db.Migrator().DropTable(&model.NotificationState{}); err != nil {
					t.Fatal(err)
				}
				if schema == "revision_only" {
					if err := db.Exec(`CREATE TABLE notification_states (
						user_id integer PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
						revision integer NOT NULL DEFAULT 0)`).Error; err != nil {
						t.Fatal(err)
					}
					wantRevision = 12
					if err := db.Exec("INSERT INTO notification_states (user_id, revision) VALUES (?, ?)", owner.ID, wantRevision).Error; err != nil {
						t.Fatal(err)
					}
				}
			case "current":
				// 故意设置不同的计数，证明重复迁移不会私自重算已有状态。
				wantRevision, wantCount = 20, 7
				state := model.NotificationState{UserID: owner.ID, Revision: wantRevision, UnreadCount: wantCount}
				if err := db.Omit("User").Create(&state).Error; err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				if err := migrateDatabase(db); err != nil {
					t.Fatal(err)
				}
			}
			var state model.NotificationState
			if err := db.First(&state, "user_id = ?", owner.ID).Error; err != nil {
				t.Fatal(err)
			}
			if state.Revision != wantRevision || state.UnreadCount != wantCount {
				t.Fatalf("unexpected state: %+v", state)
			}
			var readOnly model.NotificationState
			if err := db.First(&readOnly, "user_id = ?", member.ID).Error; err != nil {
				t.Fatal(err)
			}
			if readOnly.Revision != 0 || readOnly.UnreadCount != 0 {
				t.Fatalf("unexpected read-only state: %+v", readOnly)
			}
			var saved []model.Notification
			var emptyState model.NotificationState
			if err := db.First(&emptyState, "user_id = ?", noNotice.ID).Error; err != nil {
				t.Fatal(err)
			}
			if emptyState.Revision != 0 || emptyState.UnreadCount != 0 {
				t.Fatalf("unexpected empty state: %+v", emptyState)
			}
			if err := db.Order("id ASC").Find(&saved).Error; err != nil {
				t.Fatal(err)
			}
			if len(saved) != len(notices) {
				t.Fatal("migration changed notification count")
			}
			for i, got := range saved {
				want := notices[i]
				if got.ID != want.ID || got.ReceiverID != want.ReceiverID || got.ActorID != want.ActorID ||
					got.GroupID != want.GroupID || got.Type != want.Type || !got.CreatedAt.Equal(want.CreatedAt) ||
					(got.ReadAt == nil) != (want.ReadAt == nil) || (got.ReadAt != nil && !got.ReadAt.Equal(*want.ReadAt)) {
					t.Fatalf("migration changed notification: %+v", got)
				}
			}
		})
	}
}

func TestNotificationStateMigrationRollsBack(t *testing.T) {
	db, owner, member, group := groupSchemaFixture(t)
	if err := db.Migrator().DropTable(&model.NotificationState{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE notification_states (
		user_id integer PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
		revision integer NOT NULL DEFAULT 0)`).Error; err != nil {
		t.Fatal(err)
	}
	notice := model.Notification{ReceiverID: owner.ID, ActorID: member.ID, GroupID: group.ID, Type: model.Joined}
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&notice).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TEMP TRIGGER reject_notification_state BEFORE INSERT ON notification_states
		BEGIN SELECT RAISE(ABORT, 'test state migration failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateNotificationStateSchema(db); err == nil {
		t.Fatal("expected migration failure")
	}
	if db.Migrator().HasColumn(&model.NotificationState{}, "UnreadCount") {
		t.Fatal("failed migration left a partially upgraded schema")
	}
	if err := db.Exec("DROP TRIGGER reject_notification_state").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	var state model.NotificationState
	if err := db.First(&state, "user_id = ?", owner.ID).Error; err != nil {
		t.Fatal(err)
	}
	if state.UnreadCount != 1 || state.Revision != 0 {
		t.Fatalf("retry did not initialize state: %+v", state)
	}
}
