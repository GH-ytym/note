package internal

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"note/internal/model"
	"note/internal/notification"

	"gorm.io/gorm"
)

func TestEnqueueChangeUnreadDelta(t *testing.T) {
	for _, tc := range []struct {
		name      string
		operation string
		read      bool

		delta int64
		want  int64
	}{
		{"new_unread", "insert", false, 1, 2},
		{"update_unread", "update", false, 0, 1},
		{"mark_read", "read", false, -1, 0},
		{"update_read", "update", true, 0, 0},
		{"delete_unread", "delete", false, -1, 0},
		{"delete_read", "delete", true, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, owner, member, group := groupSchemaFixture(t)
			now := time.Now().UTC()
			n := model.Notification{ReceiverID: owner.ID, ActorID: member.ID, GroupID: group.ID,
				Type: model.Joined, CreatedAt: now}
			initialUnread := int64(1)
			if tc.read {
				n.ReadAt = &now
				initialUnread = 0
			}
			if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&n).Error; err != nil {
				t.Fatal(err)
			}
			{
				st := model.NotificationState{UserID: owner.ID, Revision: 4, UnreadCount: initialUnread}
				if err := db.Omit("User").Create(&st).Error; err != nil {
					t.Fatal(err)
				}
			}
			counts := 0
			if err := db.Callback().Query().After("gorm:query").Register("test:count_notification_queries", func(q *gorm.DB) {
				sql := strings.ToLower(q.Statement.SQL.String())
				if strings.Contains(sql, "count(") && strings.Contains(sql, "notifications") {
					counts++
				}
			}); err != nil {
				t.Fatal(err)
			}
			var change notification.Change
			err := db.Transaction(func(tx *gorm.DB) error {
				var removed []uint
				var cards []notification.Card
				switch tc.operation {
				case "insert":
					n.ID = 0
					if err := tx.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&n).Error; err != nil {
						return err
					}
				case "read":
					n.ReadAt = &now
					if err := tx.Model(&n).Update("read_at", now).Error; err != nil {
						return err
					}
				case "update":
					member.Avatar = "https://example.com/new-avatar.png"
					if err := tx.Model(&member).Update("avatar", member.Avatar).Error; err != nil {
						return err
					}
				case "delete":
					if err := tx.Delete(&n).Error; err != nil {
						return err
					}
					removed = []uint{n.ID}
				}
				if tc.operation != "delete" {
					n.Actor, n.Group = &member, &group
					card, err := notification.NewCard(n)
					if err != nil {
						return err
					}
					cards = []notification.Card{card}
				}
				var err error
				change, err = notification.EnqueueChange(tx, owner.ID, cards, removed, tc.delta)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			wantCounts, wantRevision := 0, uint64(5)
			if counts != wantCounts {
				t.Fatalf("COUNT queries=%d want %d", counts, wantCounts)
			}
			if change.UnreadCount != tc.want || change.Revision != wantRevision {
				t.Fatalf("unexpected change: %+v", change)
			}
			var state model.NotificationState
			if err := db.First(&state, "user_id = ?", owner.ID).Error; err != nil {
				t.Fatal(err)
			}
			var actual int64
			if err := db.Model(&model.Notification{}).Where("receiver_id = ? AND read_at IS NULL", owner.ID).Count(&actual).Error; err != nil {
				t.Fatal(err)
			}
			if state.UnreadCount != actual || actual != tc.want || state.Revision != wantRevision {
				t.Fatalf("state=%+v actual=%d", state, actual)
			}
			var tasks []model.Outbox
			if err := db.Find(&tasks).Error; err != nil {
				t.Fatal(err)
			}
			if len(tasks) != 1 || tasks[0].Revision != wantRevision || tasks[0].ReceiverID != owner.ID {
				t.Fatalf("unexpected tasks: %+v", tasks)
			}
			var payload notification.Change
			if err := json.Unmarshal([]byte(tasks[0].Data), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.UnreadCount != actual || payload.Revision != wantRevision || payload.Upserts == nil || payload.RemovedIDs == nil {
				t.Fatalf("unexpected payload: %+v", payload)
			}
		})
	}
}

func TestEnqueueChangeFailureRollsBack(t *testing.T) {
	const max int64 = 9223372036854775807
	for _, tc := range []struct {
		name         string
		count        int64
		revision     uint64
		delta        int64
		rejectOutbox bool
	}{
		{"negative_count", 0, 3, -1, false},
		{"count_overflow", max, 3, 1, false},
		{"revision_overflow", 0, uint64(max), 1, false},
		{"delta_min_int64", 0, 3, -max - 1, false},
		{"outbox_failure", 0, 3, 1, true},
		{"missing_state", 0, 3, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, owner, member, group := groupSchemaFixture(t)
			state := model.NotificationState{UserID: owner.ID, Revision: tc.revision, UnreadCount: tc.count}
			if tc.name != "missing_state" {
				if err := db.Omit("User").Create(&state).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.rejectOutbox {
				if err := db.Exec(`CREATE TEMP TRIGGER reject_change_outbox BEFORE INSERT ON outboxes
					BEGIN SELECT RAISE(ABORT, 'test outbox failure'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			err := db.Transaction(func(tx *gorm.DB) error {
				n := model.Notification{ReceiverID: owner.ID, ActorID: member.ID, GroupID: group.ID, Type: model.Joined}
				if err := tx.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&n).Error; err != nil {
					return err
				}
				n.Actor, n.Group = &member, &group
				card, err := notification.NewCard(n)
				if err != nil {
					return err
				}
				_, err = notification.EnqueueChange(tx, owner.ID, []notification.Card{card}, nil, tc.delta)
				return err
			})
			if err == nil {
				t.Fatal("expected failure")
			}
			var saved model.NotificationState
			if tc.name == "missing_state" {
				var states int64
				if err := db.Model(&saved).Count(&states).Error; err != nil {
					t.Fatal(err)
				}
				if states != 0 {
					t.Fatal("enqueue unexpectedly created state")
				}
			} else {
				if err := db.First(&saved, "user_id = ?", owner.ID).Error; err != nil {
					t.Fatal(err)
				}
				if saved.Revision != state.Revision || saved.UnreadCount != state.UnreadCount {
					t.Fatalf("state not rolled back: %+v", saved)
				}
			}
			for _, table := range []any{&model.Notification{}, &model.Outbox{}} {
				var count int64
				if err := db.Model(table).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatalf("%T not rolled back: %d", table, count)
				}
			}
		})
	}
}
