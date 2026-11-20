package internal

import (
	"context"
	"testing"
	"time"

	"note/internal/calendarsync"
	"note/internal/model"
	"note/internal/notification"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestCalendarOutboxDelivery(t *testing.T) {
	db := authTestDB(t)
	if err := migrateOutboxSchema(db); err != nil {
		t.Fatal(err)
	}
	s := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: s.Addr(), MaxRetries: -1})
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hub := notification.NewHub()
	defer hub.Close()
	a, _, _ := hub.Subscribe(8)
	b, _, _ := hub.Subscribe(8)
	other, _, _ := hub.Subscribe(9)
	sub, err := notification.SubscribeCldr(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- notification.RunCldrSubscriber(ctx, sub, hub) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("subscriber did not stop")
		}
	}()
	// Duplicate recipients must not produce duplicate tasks.
	if err := calendarsync.EnqueueCalendar(db, []uint{8, 8}, calendarsync.Change{
		GroupID: 3, Kind: "todo", ID: 42, Action: "upsert",
	}); err != nil {
		t.Fatal(err)
	}
	repo := notification.NewGORMRepository(db)
	cache := notification.NewRedisCache(c)
	if ok, err := cache.InitSnapshot(ctx, 8, notification.Inbox{}); err != nil || !ok {
		t.Fatalf("init: %v %v", ok, err)
	}
	// A notification in the same batch still goes through the original delivery.
	note := model.Outbox{ReceiverID: 8, Name: "notification.change", Revision: 1,
		Data: `{"revision":"1","upserts":[],"removed_ids":[],"unread_count":0}`}
	if err := db.Create(&note).Error; err != nil {
		t.Fatal(err)
	}
	d := notification.NewDispatcher(
		notification.NewDelivery(cache, notification.NewRebuilder(repo, cache)),
		calendarsync.NewDelivery(c),
	)
	if err := notification.PublishPending(ctx, repo, d); err != nil {
		t.Fatal(err)
	}
	for _, ch := range []<-chan notification.Event{a, b} {
		select {
		case e := <-ch:
			if e.Name != "cldr" || e.Data != `{"group_id":3,"kind":"todo","id":42,"action":"upsert"}` {
				t.Fatalf("unexpected event: %+v", e)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	select {
	case e := <-other:
		t.Fatalf("event leaked to another user: %+v", e)
	default:
	}
	var tasks []model.Outbox
	if err := db.Order("id").Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].PublishedAt == nil || tasks[1].PublishedAt == nil {
		t.Fatalf("expected two completed tasks: %+v", tasks)
	}
	// A failed Redis publish remains pending for the next worker round.
	if err := calendarsync.EnqueueCalendar(db, []uint{8}, calendarsync.Change{
		GroupID: 3, Kind: "event", ID: 43, Action: "remove",
	}); err != nil {
		t.Fatal(err)
	}
	s.SetError("ERR temporarily unavailable")
	if err := notification.PublishPending(ctx, repo, d); err == nil {
		t.Fatal("expected publish failure")
	}
	pending, err := repo.ListPendingOutbox(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending: %+v, %v", pending, err)
	}
	s.SetError("")
	if err := notification.PublishPending(ctx, repo, d); err != nil {
		t.Fatal(err)
	}
	pending, err = repo.ListPendingOutbox(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after retry: %+v, %v", pending, err)
	}
}
