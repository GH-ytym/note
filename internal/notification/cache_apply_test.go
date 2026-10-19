package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func applyFixture(t *testing.T) (context.Context, *miniredis.Miniredis, *redis.Client, *RedisCache, *redis.PubSub) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	s := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: s.Addr(), MaxRetries: -1, ReadTimeout: time.Second, ContextTimeoutEnabled: true})
	t.Cleanup(func() { _ = c.Close() })
	// 模式订阅也能发现误发给其他用户的消息。
	sub := c.PSubscribe(ctx, "note:notifications:v2:*:live")
	t.Cleanup(func() { _ = sub.Close() })
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	return ctx, s, c, NewRedisCache(c), sub
}

// 同一 Redis 连接顺序发送屏障消息；若前面误广播，先读到的就不会是屏障。
func assertNoApplyBroadcast(t *testing.T, ctx context.Context, c *redis.Client, sub *redis.PubSub) {
	t.Helper()
	const channel = "note:notifications:v2:{test-barrier}:live"
	if err := c.Publish(ctx, channel, "barrier").Err(); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.ReceiveMessage(ctx)
	if err != nil || msg.Channel != channel || msg.Payload != "barrier" {
		t.Fatalf("unexpected broadcast before barrier: %+v, %v", msg, err)
	}
}

func TestApplyChangeCacheHistoryAndBroadcast(t *testing.T) {
	ctx, _, c, cache, sub := applyFixture(t)
	k := notificationCacheKeys(42)
	const base = uint64(1<<53 + 1)
	initial := Inbox{Revision: base, UnreadCount: 2, Items: []Card{{ID: 101, ActorName: "旧名字"}, {ID: 102}}}
	if ok, err := cache.InitSnapshot(ctx, 42, initial); err != nil || !ok {
		t.Fatalf("init: %v %v", ok, err)
	}
	if ok, err := cache.InitSnapshot(ctx, 43, Inbox{Revision: 7, Items: []Card{{ID: 201}}}); err != nil || !ok {
		t.Fatalf("other init: %v %v", ok, err)
	}
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	change := Change{Revision: base + 1, UnreadCount: 1,
		Upserts:    []Card{{ID: 101, ActorName: "新名字", ReadAt: &now}, {ID: 103, ActorName: "张三"}},
		RemovedIDs: []uint{102}}
	result, err := cache.ApplyChange(ctx, 42, change)
	if err != nil || result != ApplyApplied {
		t.Fatalf("apply: %s %v", result, err)
	}
	if got := c.HGet(ctx, k.state, "revision").Val(); got != strconv.FormatUint(change.Revision, 10) {
		t.Fatalf("revision lost precision: %s", got)
	}
	if got := c.HGet(ctx, k.state, "unread_count").Val(); got != "1" {
		t.Fatalf("count must be replaced, got %s", got)
	}
	if c.HExists(ctx, k.state, "applying_revision").Val() {
		t.Fatal("applying marker not cleared")
	}
	if c.HExists(ctx, k.items, "102").Val() {
		t.Fatal("deleted card remains")
	}
	ids, err := c.ZRevRange(ctx, k.order, 0, -1).Result()
	if err != nil || !reflect.DeepEqual(ids, []string{"103", "101"}) {
		t.Fatalf("order: %v %v", ids, err)
	}
	for _, card := range change.Upserts {
		data, err := c.HGet(ctx, k.items, strconv.FormatUint(uint64(card.ID), 10)).Bytes()
		if err != nil {
			t.Fatal(err)
		}
		var saved Card
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(saved, card) {
			t.Fatalf("card: %+v want %+v", saved, card)
		}
	}
	rows, err := c.XRange(ctx, k.history, "-", "+").Result()
	if err != nil || len(rows) != 1 {
		t.Fatalf("history: %+v %v", rows, err)
	}
	wantJSON, _ := json.Marshal(change)
	if rows[0].Values["data"] != string(wantJSON) || rows[0].Values["revision"] != strconv.FormatUint(change.Revision, 10) || rows[0].Values["name"] != "notification.change" {
		t.Fatalf("history payload: %+v", rows[0])
	}
	message, err := sub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var event Event
	if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
		t.Fatal(err)
	}
	if message.Channel != k.live || event.ID != rows[0].ID || event.Name != "notification.change" || event.Data != string(wantJSON) {
		t.Fatalf("broadcast/history mismatch: %+v %+v", message, event)
	}
	other := notificationCacheKeys(43)
	if c.HGet(ctx, other.state, "revision").Val() != "7" || c.HLen(ctx, other.items).Val() != 1 || c.Exists(ctx, other.history).Val() != 0 {
		t.Fatal("modified another user's cache")
	}
	// 重试及迟到旧事件既不改状态，也不重复留下历史或广播。
	for _, revision := range []uint64{change.Revision, base} {
		result, err := cache.ApplyChange(ctx, 42, Change{Revision: revision, UnreadCount: 999, RemovedIDs: []uint{101}})
		if err != nil || result != ApplyIgnored {
			t.Fatalf("retry: %s %v", result, err)
		}
	}
	if c.XLen(ctx, k.history).Val() != 1 || !c.HExists(ctx, k.items, "101").Val() || c.HGet(ctx, k.state, "unread_count").Val() != "1" {
		t.Fatal("ignored change mutated cache/history")
	}
	assertNoApplyBroadcast(t, ctx, c, sub)
}

func TestApplyChangeMissingGapAndCorruption(t *testing.T) {
	for _, name := range []string{"missing", "gap", "state_type", "items_type", "order_type", "history_type", "incomplete_state", "invalid_revision", "invalid_unread", "missing_order", "unfinished_duplicate"} {
		t.Run(name, func(t *testing.T) {
			ctx, s, c, cache, sub := applyFixture(t)
			k := notificationCacheKeys(42)
			if name != "missing" {
				if ok, err := cache.InitSnapshot(ctx, 42, Inbox{Revision: 8, UnreadCount: 1, Items: []Card{{ID: 101}}}); err != nil || !ok {
					t.Fatalf("init: %v %v", ok, err)
				}
			}
			change := Change{Revision: 9, UnreadCount: 2, Upserts: []Card{{ID: 102}}}
			var setupErr error
			switch name {
			case "gap":
				change.Revision = 10
			case "state_type":
				setupErr = c.Set(ctx, k.state, "wrong", 0).Err()
			case "items_type":
				setupErr = c.Set(ctx, k.items, "wrong", 0).Err()
			case "order_type":
				setupErr = c.Set(ctx, k.order, "wrong", 0).Err()
			case "history_type":
				setupErr = c.Set(ctx, k.history, "wrong", 0).Err()
			case "incomplete_state":
				setupErr = c.HDel(ctx, k.state, "unread_count").Err()
			case "invalid_revision":
				setupErr = c.HSet(ctx, k.state, "revision", "08").Err()
			case "invalid_unread":
				setupErr = c.HSet(ctx, k.state, "unread_count", "-1").Err()
			case "missing_order":
				setupErr = c.Del(ctx, k.order).Err()
			case "unfinished_duplicate":
				change.Revision = 8
				setupErr = c.HSet(ctx, k.state, "applying_revision", "8").Err()
			}
			if setupErr != nil {
				t.Fatal(setupErr)
			}
			before := s.Dump()
			result, err := cache.ApplyChange(ctx, 42, change)
			switch name {
			case "missing":
				if err != nil || result != ApplyMissing {
					t.Fatalf("missing: %s %v", result, err)
				}
			case "gap":
				if err != nil || result != ApplyGap {
					t.Fatalf("gap: %s %v", result, err)
				}
			default:
				if err == nil {
					t.Fatalf("expected error, got %s", result)
				}
			}
			if s.Dump() != before {
				t.Fatal("rejected change modified Redis")
			}
			assertNoApplyBroadcast(t, ctx, c, sub)
		})
	}
}

func TestApplyChangeEmptyCacheDeleteAndHistoryLimit(t *testing.T) {
	ctx, _, c, cache, sub := applyFixture(t)
	k := notificationCacheKeys(42)
	if ok, err := cache.InitSnapshot(ctx, 42, Inbox{Revision: 8}); err != nil || !ok {
		t.Fatalf("init: %v %v", ok, err)
	}
	// 先准备满额历史，验证追加时裁剪历史不会裁剪当前通知。
	for i := 1; i <= 1000; i++ {
		if err := c.XAdd(ctx, &redis.XAddArgs{Stream: k.history, ID: fmt.Sprintf("%d-0", i), Values: map[string]any{"data": "old"}}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	for i, change := range []Change{
		{Revision: 9, UnreadCount: 1, Upserts: []Card{{ID: 101}}},
		{Revision: 10, UnreadCount: 0, RemovedIDs: []uint{101, 999}},
	} {
		result, err := cache.ApplyChange(ctx, 42, change)
		if err != nil || result != ApplyApplied {
			t.Fatalf("apply: %s %v", result, err)
		}
		if c.XLen(ctx, k.history).Val() != 1000 {
			t.Fatal("history limit not enforced")
		}
		if i == 0 && !c.HExists(ctx, k.items, "101").Val() {
			t.Fatal("history trimming removed current card")
		}
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event Event
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(event.Data, `"upserts":null`) || strings.Contains(event.Data, `"removed_ids":null`) {
			t.Fatalf("nil slices were not normalized: %s", event.Data)
		}
	}
	if c.HLen(ctx, k.items).Val() != 0 || c.ZCard(ctx, k.order).Val() != 0 || c.HGet(ctx, k.state, "unread_count").Val() != "0" {
		t.Fatal("delete last card failed")
	}
	assertNoApplyBroadcast(t, ctx, c, sub)
}

func TestApplyChangeHistoryFailureRequiresRebuild(t *testing.T) {
	ctx, _, c, cache, sub := applyFixture(t)
	k := notificationCacheKeys(42)
	if ok, err := cache.InitSnapshot(ctx, 42, Inbox{Revision: 8}); err != nil || !ok {
		t.Fatalf("init: %v %v", ok, err)
	}
	// Stream ID 达到最大值后，XADD * 无法继续生成 ID。
	// 故障发生在卡片写入之后，用来验证 Lua 不会自动回滚。
	if err := c.XAdd(ctx, &redis.XAddArgs{
		Stream: k.history, ID: "18446744073709551615-18446744073709551615",
		Values: map[string]any{"data": "old"},
	}).Err(); err != nil {
		t.Fatal(err)
	}
	change := Change{Revision: 9, UnreadCount: 1, Upserts: []Card{{ID: 101}}}
	if _, err := cache.ApplyChange(ctx, 42, change); err == nil {
		t.Fatal("expected history append failure")
	}
	if !c.HExists(ctx, k.items, "101").Val() || c.HGet(ctx, k.state, "applying_revision").Val() != "9" {
		t.Fatal("partial write was not marked for recovery")
	}
	if c.HGet(ctx, k.state, "revision").Val() != "8" || c.XLen(ctx, k.history).Val() != 1 {
		t.Fatal("failed append advanced revision or history")
	}
	if _, err := cache.ApplyChange(ctx, 42, change); err == nil || !strings.Contains(err.Error(), "needs rebuild") {
		t.Fatalf("retry did not require recovery: %v", err)
	}
	assertNoApplyBroadcast(t, ctx, c, sub)
}
