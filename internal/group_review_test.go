package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	apperrors "note/internal/errors"
	"note/internal/group"
	"note/internal/model"
	"note/internal/notification"
)

func TestGroupReviewValidation(t *testing.T) {
	for _, name := range []string{"not_owner", "missing", "invalid", "expired", "invitation"} {
		t.Run(name, func(t *testing.T) {
			db, owner, user, g := groupSchemaFixture(t)
			initializeJoinNotificationStates(t, db, owner, user)
			if err := db.Model(&g).Updates(map[string]any{"code": "ABC123", "policy": model.Approval}).Error; err != nil {
				t.Fatal(err)
			}
			repo := group.NewGORMRepository(db)
			req, _, err := repo.Join(context.Background(), user.ID, "ABC123")
			if err != nil {
				t.Fatal(err)
			}
			uid, id, status := owner.ID, req.ID, model.Accepted
			var want error
			switch name {
			case "not_owner":
				uid, want = user.ID, apperrors.ErrGroupReviewDenied
			case "missing":
				id, want = req.ID+100, apperrors.ErrGroupJoinRequestNotFound
			case "invalid":
				status, want = model.Cancelled, apperrors.ErrGroupReviewInvalid
			case "expired":
				want = apperrors.ErrGroupRequestExpired
				if err := db.Model(req).Update("created_at", time.Now().Add(-8*24*time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			case "invitation":
				want = apperrors.ErrGroupReviewInvalid
				if err := db.Model(req).Update("kind", model.Invitation).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := repo.Review(context.Background(), uid, id, status); !errors.Is(err, want) {
				t.Fatalf("got %v want %v", err, want)
			}
			assertJoinNotificationState(t, db, owner.ID, 1, 1)
			assertJoinNotificationState(t, db, user.ID, 0, 0)
			var count int64
			if err := db.Model(&model.Outbox{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("unexpected outbox: %d", count)
			}
		})
	}
}

func TestGroupReviewNotifications(t *testing.T) {
	for _, status := range []model.Status{model.Accepted, model.Rejected} {
		for _, read := range []bool{false, true} {
			name := string(status)
			if read {
				name += "_already_read"
			}
			t.Run(name, func(t *testing.T) {
				db, owner, user, g := groupSchemaFixture(t)
				initializeJoinNotificationStates(t, db, owner, user)
				if err := db.Model(&g).Updates(map[string]any{"code": "ABC123", "policy": model.Approval}).Error; err != nil {
					t.Fatal(err)
				}
				repo := group.NewGORMRepository(db)
				ctx := context.Background()
				req, msg, err := repo.Join(ctx, user.ID, "ABC123")
				if err != nil {
					t.Fatal(err)
				}
				rev := uint64(2)
				if read {
					// 准备已读通知及对应计数，审核后不能再次扣减。
					if err := db.Model(msg).Update("read_at", req.CreatedAt).Error; err != nil {
						t.Fatal(err)
					}
					if err := db.Model(&model.NotificationState{}).Where("user_id = ?", owner.ID).Updates(map[string]any{"unread_count": 0}).Error; err != nil {
						t.Fatal(err)
					}
				}
				if err := repo.Review(ctx, owner.ID, req.ID, status); err != nil {
					t.Fatal(err)
				}
				assertJoinNotificationState(t, db, owner.ID, rev, 0)
				assertJoinNotificationState(t, db, user.ID, 1, 1)
				var notices []model.Notification
				if err := db.Order("id").Find(&notices).Error; err != nil {
					t.Fatal(err)
				}
				if len(notices) != 2 || notices[0].ID != msg.ID || notices[0].ReadAt == nil || notices[1].ReceiverID != user.ID || notices[1].ReadAt != nil || notices[1].Type != model.JoinReviewed {
					t.Fatalf("unexpected notices: %+v", notices)
				}
				if read && !notices[0].ReadAt.Equal(req.CreatedAt) {
					t.Fatal("replaced original read time")
				}
				var tasks []model.Outbox
				if err := db.Order("id").Find(&tasks).Error; err != nil {
					t.Fatal(err)
				}
				if len(tasks) != 3 {
					t.Fatalf("tasks=%d", len(tasks))
				}
				for i, task := range tasks[1:] {
					var change notification.Change
					if err := json.Unmarshal([]byte(task.Data), &change); err != nil {
						t.Fatal(err)
					}
					if len(change.Upserts) != 1 || change.Upserts[0].ID != notices[i].ID || change.Upserts[0].RequestStatus != status {
						t.Fatalf("bad change: %+v", change)
					}
				}
				var count int64
				db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", g.ID, user.ID).Count(&count)
				if (count == 1) != (status == model.Accepted) {
					t.Fatalf("members=%d", count)
				}
				if err := repo.Review(ctx, owner.ID, req.ID, status); !errors.Is(err, apperrors.ErrGroupReviewConflict) {
					t.Fatalf("repeat review: %v", err)
				}
				db.Model(&model.Outbox{}).Count(&count)
				if count != 3 {
					t.Fatalf("repeat created outbox: %d", count)
				}
			})
		}
	}
}

func TestGroupReviewRollback(t *testing.T) {
	db, owner, user, g := groupSchemaFixture(t)
	initializeJoinNotificationStates(t, db, owner, user)
	if err := db.Model(&g).Updates(map[string]any{"code": "ABC123", "policy": model.Approval}).Error; err != nil {
		t.Fatal(err)
	}
	repo := group.NewGORMRepository(db)
	ctx := context.Background()
	req, msg, err := repo.Join(ctx, user.ID, "ABC123")
	if err != nil {
		t.Fatal(err)
	}
	// 在最后一条 Outbox 插入时失败，验证前面双方的修改全部回滚。
	if err := db.Exec(fmt.Sprintf(`CREATE TRIGGER fail_review_outbox BEFORE INSERT ON outboxes
		WHEN NEW.receiver_id = %d BEGIN SELECT RAISE(ABORT, 'test failure'); END`, user.ID)).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.Review(ctx, owner.ID, req.ID, model.Accepted); err == nil {
		t.Fatal("expected failure")
	}
	if err := db.First(req, req.ID).Error; err != nil {
		t.Fatal(err)
	}
	if req.Status != model.Pending || req.HandledAt != nil {
		t.Fatalf("request not rolled back: %+v", req)
	}
	if err := db.First(msg, msg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if msg.ReadAt != nil {
		t.Fatal("read state not rolled back")
	}
	assertJoinNotificationState(t, db, owner.ID, 1, 1)
	assertJoinNotificationState(t, db, user.ID, 0, 0)
	for _, table := range []any{&model.Notification{}, &model.Outbox{}} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("unexpected rows: %d", count)
		}
	}
	var count int64
	db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", g.ID, user.ID).Count(&count)
	if count != 0 {
		t.Fatal("membership not rolled back")
	}
}
