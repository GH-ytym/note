package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"note/internal/group"
	"note/internal/model"
	"note/internal/notification"
)

func TestGroupJoinCreatesOutbox(t *testing.T) {
	for _, policy := range []model.GroupPolicy{model.Public, model.Approval} {
		t.Run(string(policy), func(t *testing.T) {
			db, owner, member, item := groupSchemaFixture(t)
			if err := db.Model(&item).Updates(map[string]any{"policy": policy, "code": "ABC123"}).Error; err != nil {
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

			repo := group.NewGORMRepository(db)
			pending, notice, err := repo.Join(context.Background(), member.ID, "ABC123")
			if err != nil || notice == nil {
				t.Fatalf("join: notice=%+v err=%v", notice, err)
			}
			var task model.Outbox
			if err := db.First(&task).Error; err != nil {
				t.Fatal(err)
			}
			if task.ID == 0 || task.ReceiverID != owner.ID || task.Name != "notification.created" ||
				task.PublishedAt != nil || !task.CreatedAt.Equal(notice.CreatedAt) {
				t.Fatalf("unexpected pending task: %+v", task)
			}
			var card notification.Card
			if err := json.Unmarshal([]byte(task.Data), &card); err != nil {
				t.Fatal(err)
			}
			if card.ID != notice.ID || card.ActorID != member.ID ||
				card.ActorName != fmt.Sprintf("%s#%05d", member.Username, member.Suffix) ||
				card.ActorAvatar != member.Avatar || card.GroupID != item.ID || card.GroupName != item.Name ||
				!card.CreatedAt.Equal(notice.CreatedAt) || card.ReadAt != nil {
				t.Fatalf("unexpected card snapshot: %+v", card)
			}
			if policy == model.Public {
				if pending != nil || notice.JoinRequestID != nil || card.Type != model.Joined || card.RequestStatus != "" {
					t.Fatalf("public join contains an application: %+v", card)
				}
			} else if pending == nil || notice.JoinRequestID == nil || *notice.JoinRequestID != pending.ID ||
				card.Type != model.JoinRequested || card.RequestStatus != model.Pending ||
				pending.Kind != model.Application || pending.SenderID != member.ID ||
				pending.ReceiverID != owner.ID || pending.CreatedAt.IsZero() || pending.HandledAt != nil {
				t.Fatalf("approval join has invalid application snapshot: %+v", card)
			}
			assertNotificationCardFields(t, []byte(task.Data), card.RequestStatus)

			// 已入群或复用待审核申请时，不再创建通知和投递任务。
			again, repeatedNotice, err := repo.Join(context.Background(), member.ID, "ABC123")
			if err != nil || repeatedNotice != nil || (pending != nil && (again == nil || again.ID != pending.ID)) {
				t.Fatalf("repeat join: pending=%+v notice=%+v err=%v", again, repeatedNotice, err)
			}
			for _, table := range []any{&model.Notification{}, &model.Outbox{}} {
				var count int64
				if err := db.Model(table).Count(&count).Error; err != nil || count != 1 {
					t.Fatalf("repeat join duplicated %T: count=%d err=%v", table, count, err)
				}
			}

			// 后续资料或申请状态发生变化，这次事件仍保存创建时的内容。
			if err := db.Model(&item).Update("name", "新群名").Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&member).Update("nickname", "新昵称").Error; err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			if err := db.Model(&model.Notification{}).Where("id = ?", notice.ID).Update("read_at", now).Error; err != nil {
				t.Fatal(err)
			}
			if pending != nil {
				if err := db.Model(pending).Updates(map[string]any{"status": model.Rejected, "handled_at": now}).Error; err != nil {
					t.Fatal(err)
				}
			}
			var saved model.Outbox
			if err := db.First(&saved, task.ID).Error; err != nil || saved.Data != task.Data {
				t.Fatalf("later changes rewrote event snapshot: %+v err=%v", saved, err)
			}
		})
	}
}

func TestGroupJoinRollsBackWhenOutboxInsertFails(t *testing.T) {
	for _, policy := range []model.GroupPolicy{model.Public, model.Approval} {
		t.Run(string(policy), func(t *testing.T) {
			db, owner, member, item := groupSchemaFixture(t)
			if err := db.Model(&item).Updates(map[string]any{"policy": policy, "code": "ABC123"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.GroupMember{GroupID: item.ID, UserID: owner.ID}).Error; err != nil {
				t.Fatal(err)
			}
			startsAt := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
			content := "已有内容"
			todo := model.Todo{GroupID: item.ID, CreatorID: owner.ID, Title: "已有 Todo", Content: &content, StartsAt: &startsAt, RepeatMode: model.RepeatOnce}
			if err := db.Omit("Group", "Creator", "Members").Create(&todo).Error; err != nil {
				t.Fatal(err)
			}
			event := model.Event{GroupID: item.ID, CreatorID: owner.ID, Title: "已有 Event", StartsAt: startsAt, EndsAt: startsAt.Add(time.Hour)}
			if err := db.Omit("Group", "Creator", "Members").Create(&event).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.TodoMember{TodoID: todo.ID, UserID: owner.ID, Role: model.TodoEditor}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.EventMember{EventID: event.ID, UserID: owner.ID, Role: model.EventEditor}).Error; err != nil {
				t.Fatal(err)
			}
			// 只在临时测试库中让最后的任务写入失败，验证前面的写入全部回滚。
			if err := db.Exec(`CREATE TEMP TRIGGER reject_outbox
				BEFORE INSERT ON outboxes
				BEGIN SELECT RAISE(ABORT, 'test outbox failure'); END`).Error; err != nil {
				t.Fatal(err)
			}
			pending, notice, err := group.NewGORMRepository(db).Join(context.Background(), member.ID, "ABC123")
			if err == nil || !strings.Contains(err.Error(), "create notification outbox task") || pending != nil || notice != nil {
				t.Fatalf("failed outbox insertion did not fail join: pending=%+v notice=%+v err=%v", pending, notice, err)
			}
			for _, table := range []any{&model.GroupMember{}, &model.TodoMember{}, &model.EventMember{}} {
				var count int64
				if err := db.Model(table).Where("user_id = ?", member.ID).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("failed join left partial %T: count=%d err=%v", table, count, err)
				}
				if err := db.Model(table).Where("user_id = ?", owner.ID).Count(&count).Error; err != nil || count != 1 {
					t.Fatalf("failed join changed owner data in %T: count=%d err=%v", table, count, err)
				}
			}
			for _, table := range []any{&model.GroupJoinRequest{}, &model.Notification{}, &model.Outbox{}} {
				var count int64
				if err := db.Model(table).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("failed join left partial %T: count=%d err=%v", table, count, err)
				}
			}
		})
	}
}

func TestOutboxSchemaAndRepeatMigration(t *testing.T) {
	db, owner, member, item := groupSchemaFixture(t)
	notice := model.Notification{ReceiverID: owner.ID, ActorID: member.ID, GroupID: item.ID, Type: model.Joined}
	if err := db.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&notice).Error; err != nil {
		t.Fatal(err)
	}
	// 模拟升级前已有通知、但尚未建立投递任务表的数据库。
	if err := db.Migrator().DropTable(&model.Outbox{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.Outbox{}) || !db.Migrator().HasIndex(&model.Outbox{}, "PublishedAt") {
		t.Fatal("startup did not create outboxes and its pending-task index")
	}
	createdAt := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	publishedAt := createdAt.Add(time.Minute)
	tasks := []model.Outbox{
		{ReceiverID: 1, Name: "notification.created", Data: `{"id":10}`, CreatedAt: createdAt},
		{ReceiverID: 2, Name: "notification.created", Data: `{"id":20}`, CreatedAt: createdAt, PublishedAt: &publishedAt},
	}
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateDatabase(db); err != nil {
			t.Fatal(err)
		}
	}
	var saved []model.Outbox
	if err := db.Order("id ASC").Find(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if len(saved) != len(tasks) {
		t.Fatalf("repeat migration changed task count: %+v", saved)
	}
	for i, task := range saved {
		want := tasks[i]
		if task.ID != want.ID || task.ReceiverID != want.ReceiverID || task.Name != want.Name || task.Data != want.Data ||
			!task.CreatedAt.Equal(want.CreatedAt) || (task.PublishedAt == nil) != (want.PublishedAt == nil) ||
			(task.PublishedAt != nil && !task.PublishedAt.Equal(*want.PublishedAt)) {
			t.Fatalf("repeat migration changed task: got %+v want %+v", task, want)
		}
	}
	var savedNotice model.Notification
	if err := db.First(&savedNotice, notice.ID).Error; err != nil || savedNotice.ReceiverID != owner.ID ||
		savedNotice.ActorID != member.ID || savedNotice.GroupID != item.ID || !savedNotice.CreatedAt.Equal(notice.CreatedAt) {
		t.Fatalf("outbox migration changed existing notification: %+v err=%v", savedNotice, err)
	}
}
