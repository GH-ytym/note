package internal

import (
	"context"
	"errors"
	"strconv"
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
	publisher := notification.NewRedisPublisher(client)
	repo := &failFirstOutboxMark{
		Repository: notification.NewGORMRepository(db),
		failed:     make(chan struct{}),
		fail:       true,
	}

	first := model.Outbox{
		ReceiverID: 8, Name: "notification.created", Data: `{"id":101}`,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		notification.RunOutboxWorker(ctx, repo, publisher)
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
	const historyKey = "note:notifications:{8}:history"
	if count, err := client.XLen(context.Background(), historyKey).Result(); err != nil || count != 1 {
		t.Fatalf("retry duplicated history: count=%d err=%v", count, err)
	}

	// 在第一轮之后才写入第二个任务，验证 worker 会继续查询和投递。
	second := model.Outbox{
		ReceiverID: 8, Name: "notification.created", Data: `{"id":102}`,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	waitOutboxPublished(t, db, second.ID)
	if count, err := client.XLen(context.Background(), historyKey).Result(); err != nil || count != 2 {
		t.Fatalf("new task history: count=%d err=%v", count, err)
	}
	for _, taskID := range []uint{first.ID, second.ID} {
		id, err := client.HGet(context.Background(), "note:notifications:{8}:sent", strconv.FormatUint(uint64(taskID), 10)).Result()
		if err != nil || id == "" {
			t.Fatalf("task %d missing from sent hash: id=%q err=%v", taskID, id, err)
		}
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
