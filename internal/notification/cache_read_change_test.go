package notification

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func TestApplyReadChange(t *testing.T) {
	for _, missing := range []bool{false, true} {
		ctx, _, client, cache, sub := applyFixture(t)
		id := uint(103)
		cards := []Card{{ID: 101, ActorName: "unchanged"}, {ID: id, ActorName: "second notification"}}
		if ok, err := cache.InitSnapshot(ctx, 8, Inbox{Revision: 12, UnreadCount: 2, Items: cards}); err != nil || !ok {
			t.Fatalf("init: %v %v", ok, err)
		}
		k := notificationCacheKeys(8)
		ids := []uint{101, id}
		if missing {
			ids = append(ids, 999)
		}
		now := time.Date(2026, 11, 14, 10, 0, 0, 123000000, time.UTC)
		change := Change{Revision: 13, ReadIDs: ids, ReadAt: &now, UnreadCount: 0}
		result, err := cache.ApplyChange(ctx, 8, change)
		if err != nil {
			t.Fatal(err)
		}
		if missing {
			if result != ApplyRebuild {
				t.Fatal(result)
			}
			if client.HGet(ctx, k.state, "revision").Val() != "12" {
				t.Fatal("partial version advance")
			}
			var card Card
			if err := json.Unmarshal([]byte(client.HGet(ctx, k.items, "101").Val()), &card); err != nil || card.ReadAt != nil {
				t.Fatalf("partial read: %+v %v", card, err)
			}
			assertNoApplyBroadcast(t, ctx, client, sub)
			continue
		}
		if result != ApplyApplied {
			t.Fatal(result)
		}
		for _, old := range cards {
			var card Card
			if err := json.Unmarshal([]byte(client.HGet(ctx, k.items, strconv.FormatUint(uint64(old.ID), 10)).Val()), &card); err != nil {
				t.Fatal(err)
			}
			if card.ID != old.ID || card.ActorName != old.ActorName || card.ReadAt == nil || !card.ReadAt.Equal(now) {
				t.Fatalf("bad card: %+v", card)
			}
		}
		if client.HGet(ctx, k.state, "unread_count").Val() != "0" || client.ZCard(ctx, k.order).Val() != 2 {
			t.Fatal("state/order changed incorrectly")
		}
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event Event
		var got Change
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(event.Data), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Upserts) != 0 || len(got.ReadIDs) != 2 || got.ReadAt == nil || !got.ReadAt.Equal(now) {
			t.Fatalf("bad event: %+v", got)
		}
		if result, err := cache.ApplyChange(ctx, 8, change); err != nil || result != ApplyIgnored {
			t.Fatalf("retry: %v %v", result, err)
		}
		assertNoApplyBroadcast(t, ctx, client, sub)
	}
}
