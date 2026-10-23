package notification

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestInstallSnapshot(t *testing.T) {
	ctx := context.Background()
	getLease := func(t *testing.T, c *RedisCache) *lease {
		t.Helper()
		l, err := c.beginRebuild(ctx, 7)
		if err != nil || l == nil {
			t.Fatalf("lease=%v err=%v", l, err)
		}
		return l
	}

	t.Run("replace partial cache and publish baseline", func(t *testing.T) {
		c, _ := newLeaseCache(t)
		k := notificationCacheKeys(7)
		if err := c.client.HSet(ctx, k.state, "revision", "10", "unread_count", "3", "applying_revision", "11", "epoch", "old").Err(); err != nil {
			t.Fatal(err)
		}
		if err := c.client.HSet(ctx, k.items, "99", "old card").Err(); err != nil {
			t.Fatal(err)
		}
		if err := c.client.XAdd(ctx, &redis.XAddArgs{Stream: k.history, Values: map[string]any{"name": "old"}}).Err(); err != nil {
			t.Fatal(err)
		}
		sub := c.client.Subscribe(ctx, k.live)
		defer sub.Close()
		if _, err := sub.Receive(ctx); err != nil {
			t.Fatal(err)
		}
		l := getLease(t, c)
		inbox := Inbox{Revision: 12, UnreadCount: 1, Items: []Card{{ID: 101}, {ID: 102}}}
		ok, err := c.installSnapshot(ctx, l, inbox)
		if err != nil || !ok {
			t.Fatalf("installed=%v err=%v", ok, err)
		}
		state, err := c.client.HGetAll(ctx, k.state).Result()
		if err != nil {
			t.Fatal(err)
		}
		if state["revision"] != "12" || state["unread_count"] != "1" || state["base_revision"] != "12" || state["base_id"] == "" || state["epoch"] == "" || state["epoch"] == "old" {
			t.Fatalf("state=%v", state)
		}
		if _, found := state["applying_revision"]; found {
			t.Fatal("unfinished marker remains")
		}
		items, err := c.client.HGetAll(ctx, k.items).Result()
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 {
			t.Fatalf("items=%v", items)
		}
		for _, card := range inbox.Items {
			data, _ := json.Marshal(card)
			key := "101"
			if card.ID == 102 {
				key = "102"
			}
			if items[key] != string(data) {
				t.Fatalf("wrong card %s", key)
			}
		}
		ids, err := c.client.ZRange(ctx, k.order, 0, -1).Result()
		if err != nil || !reflect.DeepEqual(ids, []string{"101", "102"}) {
			t.Fatalf("order=%v err=%v", ids, err)
		}
		rows, err := c.client.XRange(ctx, k.history, "-", "+").Result()
		if err != nil || len(rows) != 1 {
			t.Fatalf("history=%v err=%v", rows, err)
		}
		if rows[0].ID != state["base_id"] || rows[0].Values["name"] != "notification.reset" {
			t.Fatal("wrong baseline")
		}
		msgCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		msg, err := sub.ReceiveMessage(msgCtx)
		if err != nil {
			t.Fatal(err)
		}
		var event Event
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			t.Fatal(err)
		}
		var reset map[string]string
		if err := json.Unmarshal([]byte(event.Data), &reset); err != nil {
			t.Fatal(err)
		}
		if event.ID != state["base_id"] || event.Name != "notification.reset" || reset["epoch"] != state["epoch"] || reset["revision"] != "12" {
			t.Fatalf("event=%+v data=%v", event, reset)
		}
		if got := c.client.Get(ctx, rebuildKey(7)).Val(); got != l.token {
			t.Fatal("install changed lease")
		}
	})

	t.Run("empty cold snapshot", func(t *testing.T) {
		c, _ := newLeaseCache(t)
		ok, err := c.installSnapshot(ctx, getLease(t, c), Inbox{})
		if err != nil || !ok {
			t.Fatalf("installed=%v err=%v", ok, err)
		}
		k := notificationCacheKeys(7)
		if c.client.HGet(ctx, k.state, "revision").Val() != "0" || c.client.XLen(ctx, k.history).Val() != 1 {
			t.Fatal("missing empty snapshot baseline")
		}
	})

	for _, mode := range []string{"expired", "state changed", "state appeared", "snapshot behind"} {
		t.Run(mode, func(t *testing.T) {
			c, m := newLeaseCache(t)
			k := notificationCacheKeys(7)
			if mode != "state appeared" {
				if err := c.client.HSet(ctx, k.state, "revision", "10", "unread_count", "0").Err(); err != nil {
					t.Fatal(err)
				}
			}
			l := getLease(t, c)
			switch mode {
			case "expired":
				m.FastForward(rebuildTTL + time.Second)
			case "state changed", "state appeared":
				if err := c.client.HSet(ctx, k.state, "revision", "11", "unread_count", "0").Err(); err != nil {
					t.Fatal(err)
				}
			}
			before := c.client.HGetAll(ctx, k.state).Val()
			rev := uint64(12)
			if mode == "snapshot behind" {
				rev = 9
			}
			ok, err := c.installSnapshot(ctx, l, Inbox{Revision: rev})
			if ok || (err != nil) != (mode == "snapshot behind") {
				t.Fatalf("installed=%v err=%v", ok, err)
			}
			if after := c.client.HGetAll(ctx, k.state).Val(); !reflect.DeepEqual(before, after) {
				t.Fatalf("state changed: %v -> %v", before, after)
			}
			if c.client.Exists(ctx, k.history).Val() != 0 {
				t.Fatal("rejected install wrote history")
			}
		})
	}
}
