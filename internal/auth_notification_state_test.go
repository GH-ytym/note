package internal

import (
	"context"
	"errors"
	"testing"

	"note/internal/auth"
	apperrors "note/internal/errors"
	"note/internal/model"
	"note/internal/notification"

	"gorm.io/gorm"
)

func TestUserCreationInitializesNotificationState(t *testing.T) {
	db, owner, _, group := groupSchemaFixture(t)
	repo := auth.NewGORMRepository(db)
	u := model.User{Username: "newuser", Suffix: 12345, Email: "new@example.com", PasswordHash: "test"}
	if err := repo.Create(context.Background(), &u); err != nil {
		t.Fatal(err)
	}
	var state model.NotificationState
	if err := db.First(&state, "user_id = ?", u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if state.Revision != 0 || state.UnreadCount != 0 {
		t.Fatalf("unexpected initial state: %+v", state)
	}

	// 账号冲突和邮箱冲突的原有语义不变，不能重置已有状态。
	if err := db.Model(&state).Updates(map[string]any{"revision": 4, "unread_count": 0}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, email string
		want        error
	}{
		{"newuser", "other@example.com", apperrors.ErrAccountTaken},
		{"other", "new@example.com", apperrors.ErrEmailTaken},
	} {
		duplicate := model.User{Username: tc.name, Suffix: 12345, Email: tc.email, PasswordHash: "test"}
		if err := repo.Create(context.Background(), &duplicate); !errors.Is(err, tc.want) {
			t.Fatalf("got %v want %v", err, tc.want)
		}
	}
	if err := db.First(&state, "user_id = ?", u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if state.Revision != 4 || state.UnreadCount != 0 {
		t.Fatal("duplicate creation changed state")
	}

	// 第一条通知直接从已有计数 0 增加到 1。
	if err := db.Transaction(func(tx *gorm.DB) error {
		n := model.Notification{ReceiverID: u.ID, ActorID: owner.ID, GroupID: group.ID, Type: model.Joined}
		if err := tx.Omit("Receiver", "Actor", "Group", "JoinRequest").Create(&n).Error; err != nil {
			return err
		}
		n.Actor, n.Group = &owner, &group
		card, err := notification.NewCard(n)
		if err != nil {
			return err
		}
		_, err = notification.EnqueueChange(tx, u.ID, []notification.Card{card}, nil, 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&state, "user_id = ?", u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if state.Revision != 5 || state.UnreadCount != 1 {
		t.Fatalf("unexpected first notification state: %+v", state)
	}
}

func TestUserCreationRollsBackWhenNotificationStateFails(t *testing.T) {
	db := authTestDB(t)
	if err := migrateDatabase(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TEMP TRIGGER reject_registration_state BEFORE INSERT ON notification_states
		BEGIN SELECT RAISE(ABORT, 'test initialization failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	u := model.User{Username: "newuser", Suffix: 12345, Email: "new@example.com", PasswordHash: "test"}
	if err := auth.NewGORMRepository(db).Create(context.Background(), &u); err == nil {
		t.Fatal("expected creation failure")
	}
	for _, table := range []any{&model.User{}, &model.NotificationState{}} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%T not rolled back", table)
		}
	}
}
