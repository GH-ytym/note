package internal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"note/internal/model"
	"note/internal/notification"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// 模拟 Redis 已经成功发布，但数据库第一次标记失败。
type failFirstOutboxMark struct {
	notification.Repository
	failed chan struct{}
	fail   bool
}

func (r *failFirstOutboxMark) MarkOutboxPublished(ctx context.Context, taskID uint, at time.Time) error {
	if r.fail {
		r.fail = false
		close(r.failed)
		return errors.New("test: temporarily unable to mark outbox")
	}
	return r.Repository.MarkOutboxPublished(ctx, taskID, at)
}

func waitOutboxPublished(t *testing.T, db *gorm.DB, taskID uint) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var task model.Outbox
		if err := db.WithContext(ctx).First(&task, taskID).Error; err != nil {
			t.Fatalf("load outbox %d: %v", taskID, err)
		}
		if task.PublishedAt != nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("outbox %d was not published: %v", taskID, ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestOutboxWorkerRetriesAndPublishesNewTasks(t *testing.T) {
	db := authTestDB(t)
	if err := migrateOutboxSchema(db); err != nil {
		t.Fatal(err)
	}
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: server.Addr(), MaxRetries: -1, ContextTimeoutEnabled: true,
	})
	t.Cleanup(func() { _ = client.Close() })
	cache := notification.NewRedisCache(client)
	if ok, err := cache.InitSnapshot(context.Background(), 8, notification.Inbox{}); err != nil || !ok {
		t.Fatalf("init: %v %v", ok, err)
	}
	repo := &failFirstOutboxMark{
		Repository: notification.NewGORMRepository(db),
		failed:     make(chan struct{}),
		fail:       true,
	}

	delivery := notification.NewDelivery(cache, notification.NewRebuilder(repo, cache))

	first := model.Outbox{
		ReceiverID: 8, Revision: 1, Name: "notification.change", Data: `{"revision":"1","upserts":[{"id":101}],"removed_ids":[],"unread_count":1}`,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		notification.RunOutboxWorker(ctx, repo, delivery)
	}()
	stopWorker := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("outbox worker did not stop after cancellation")
		}
	}
	// 必须先停止 worker，再让其他 cleanup 关闭 Redis 和 SQLite。
	t.Cleanup(stopWorker)

	select {
	case <-repo.failed:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not attempt the first task immediately")
	}
	waitOutboxPublished(t, db, first.ID)
	const historyKey = "note:notifications:v2:{8}:history"
	if count, err := client.XLen(context.Background(), historyKey).Result(); err != nil || count != 1 {
		t.Fatalf("retry duplicated history: count=%d err=%v", count, err)
	}

	// 在第一轮之后才写入第二个任务，验证 worker 会继续查询和投递。
	second := model.Outbox{
		ReceiverID: 8, Revision: 2, Name: "notification.change", Data: `{"revision":"2","upserts":[{"id":102}],"removed_ids":[],"unread_count":2}`,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	waitOutboxPublished(t, db, second.ID)
	if count, err := client.XLen(context.Background(), historyKey).Result(); err != nil || count != 2 {
		t.Fatalf("new task history: count=%d err=%v", count, err)
	}
	rows, err := client.XRange(context.Background(), historyKey, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		var change notification.Change
		if err := json.Unmarshal([]byte(row.Values["data"].(string)), &change); err != nil {
			t.Fatal(err)
		}
		if change.Revision != uint64(i+1) || row.Values["name"] != "notification.change" {
			t.Fatalf("wrong history: %+v", row)
		}
	}
	if got := client.HGet(context.Background(), "note:notifications:v2:{8}:state", "unread_count").Val(); got != "2" {
		t.Fatalf("retry changed unread count: %s", got)
	}

	stopWorker()
}

func TestOutboxWorkerAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		// 已取消时不应访问 repo 或 publisher。
		notification.RunOutboxWorker(ctx, nil, nil)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("already canceled worker did not exit")
	}
}

type workerSnapshot struct {
	load func(context.Context, uint) (notification.Inbox, error)
}

func (r workerSnapshot) LoadSnapshot(ctx context.Context, uid uint) (notification.Inbox, error) {
	return r.load(ctx, uid)
}

func TestPublishPendingKeepsUnfinishedTasks(t *testing.T) {
	for _, name := range []string{"busy", "load_failure", "redis_failure", "install_failure", "conflict", "snapshot_behind"} {
		t.Run(name, func(t *testing.T) {
			db := authTestDB(t)
			if err := migrateOutboxSchema(db); err != nil {
				t.Fatal(err)
			}
			s := miniredis.RunT(t)
			c := redis.NewClient(&redis.Options{Addr: s.Addr(), MaxRetries: -1})
			t.Cleanup(func() { _ = c.Close() })
			ctx := context.Background()
			cache := notification.NewRedisCache(c)
			repo := notification.NewGORMRepository(db)
			task := model.Outbox{ReceiverID: 8, Revision: 2, Name: "notification.change",
				Data: `{"revision":"2","upserts":[],"removed_ids":[],"unread_count":0}`}
			if err := db.Create(&task).Error; err != nil {
				t.Fatal(err)
			}
			const prefix = "note:notifications:v2:{8}"
			if name == "busy" {
				if err := c.Set(ctx, prefix+":rebuild", "other-owner", time.Minute).Err(); err != nil {
					t.Fatal(err)
				}
			}
			if name == "redis_failure" {
				s.SetError("ERR injected failure")
			}
			loads := 0
			loader := workerSnapshot{load: func(context.Context, uint) (notification.Inbox, error) {
				loads++
				if name == "load_failure" {
					return notification.Inbox{}, errors.New("injected snapshot failure")
				}
				if name == "conflict" {
					if err := c.HSet(ctx, prefix+":state", "revision", "3", "unread_count", "0").Err(); err != nil {
						t.Fatal(err)
					}
				}
				if name == "install_failure" {
					s.SetError("ERR injected install failure")
				}
				rev := uint64(2)
				if name == "snapshot_behind" {
					rev = 1
				}
				return notification.Inbox{Revision: rev}, nil
			}}
			delivery := notification.NewDelivery(cache, notification.NewRebuilder(loader, cache))
			err := notification.PublishPending(ctx, repo, delivery)
			wantErr := name != "busy" && name != "conflict"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v wantErr=%v", err, wantErr)
			}
			if (name == "busy" || name == "redis_failure") && loads != 0 {
				t.Fatal("unexpected SQL snapshot load")
			}
			var saved model.Outbox
			if err := db.First(&saved, task.ID).Error; err != nil {
				t.Fatal(err)
			}
			if saved.PublishedAt != nil {
				t.Fatal("unfinished task marked published")
			}
			s.SetError("")
			if name == "conflict" && c.HGet(ctx, prefix+":state", "revision").Val() != "3" {
				t.Fatal("overwrote concurrent state")
			}
		})
	}
}
