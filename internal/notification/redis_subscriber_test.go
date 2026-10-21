package notification

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestNewRedisSubscriberFanoutAndValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	s := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	hub := NewHub()
	t.Cleanup(hub.Close)
	first, unsub1, err := hub.Subscribe(42)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsub1)
	second, unsub2, err := hub.Subscribe(42)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsub2)
	other, unsub3, err := hub.Subscribe(43)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsub3)
	sub, err := SubscribeRedis(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- RunRedisSubscriber(ctx, sub, hub) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("subscriber stopped: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("subscriber did not stop")
		}
	})
	valid := Event{ID: "1-0", Name: "notification.change", Data: `{"revision":"1","upserts":[],"removed_ids":[],"unread_count":0}`}
	publish := func(channel string, payload any) {
		t.Helper()
		if err := c.Publish(ctx, channel, payload).Err(); err != nil {
			t.Fatal(err)
		}
	}
	for _, bad := range []Event{
		{ID: "", Name: valid.Name, Data: valid.Data},
		{ID: "1-0", Name: "notification.created", Data: valid.Data},
		{ID: "1-0", Name: valid.Name, Data: "not json"},
		{ID: "1-0", Name: valid.Name, Data: `{"revision":"0","upserts":[],"removed_ids":[],"unread_count":0}`},
		{ID: "1-0", Name: valid.Name, Data: `{"revision":"1","upserts":null,"removed_ids":[],"unread_count":0}`},
		{ID: "1-0", Name: valid.Name, Data: `{"revision":"1","upserts":[],"removed_ids":[],"unread_count":-1}`},
	} {
		data, err := json.Marshal(bad)
		if err != nil {
			t.Fatal(err)
		}
		publish(notificationCacheKeys(42).live, string(data))
	}
	publish(notificationCacheKeys(42).live, "not json")
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	publish("note:notifications:v2:{042}:live", string(data))
	publish("note:notifications:{42}:live", string(data))
	// 实际 Lua 产生有效广播。它也充当此前无效消息的顺序屏障。
	cache := NewRedisCache(c)
	if ok, err := cache.InitSnapshot(ctx, 42, Inbox{}); err != nil || !ok {
		t.Fatalf("init: %v %v", ok, err)
	}
	change := Change{Revision: 1, UnreadCount: 1, Upserts: []Card{{ID: 101}}, RemovedIDs: []uint{}}
	if result, err := cache.ApplyChange(ctx, 42, change); err != nil || result != ApplyApplied {
		t.Fatalf("apply: %s %v", result, err)
	}
	rows, err := c.XRange(ctx, notificationCacheKeys(42).history, "-", "+").Result()
	if err != nil || len(rows) != 1 {
		t.Fatalf("history: %+v %v", rows, err)
	}
	wantData, err := json.Marshal(change)
	if err != nil {
		t.Fatal(err)
	}
	want := Event{ID: rows[0].ID, Name: valid.Name, Data: string(wantData)}
	for _, window := range []<-chan Event{first, second} {
		select {
		case got := <-window:
			if got != want {
				t.Fatalf("wrong event or invalid event forwarded: %+v", got)
			}
		case <-ctx.Done():
			t.Fatal("valid event did not reach Hub")
		}
	}
	select {
	case event := <-other:
		t.Fatalf("cross-user notification: %+v", event)
	default:
	}
}

func TestReceiverFromChannel(t *testing.T) {
	if user, err := receiverFromChannel("note:notifications:v2:{42}:live"); err != nil || user != 42 {
		t.Fatalf("valid channel: %d %v", user, err)
	}
	for _, channel := range []string{
		"note:notifications:{42}:live", "note:notifications:v2:{0}:live",
		"note:notifications:v2:{042}:live", "note:notifications:v2:{-1}:live",
		"note:notifications:v2:{abc}:live", "note:notifications:v2:{}:live",
		"note:notifications:v2:{18446744073709551616}:live", "note:notifications:v2:{42}:other",
	} {
		if _, err := receiverFromChannel(channel); err == nil {
			t.Fatalf("accepted invalid channel %q", channel)
		}
	}
}
