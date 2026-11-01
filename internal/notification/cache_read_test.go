package notification

import (
	"context"
	"testing"
)

func TestReadPage(t *testing.T) {
	ctx := context.Background()
	c, _ := newLeaseCache(t)
	if _, ok, err := c.ReadPage(ctx, 8, 0, 2); err != nil || ok {
		t.Fatalf("missing: %v %v", ok, err)
	}
	ls, err := c.beginRebuild(ctx, 8)
	if err != nil || ls == nil {
		t.Fatalf("lease: %v %v", ls, err)
	}
	inbox := Inbox{Revision: 12, UnreadCount: 2, Items: []Card{{ID: 108}, {ID: 104}, {ID: 101}}}
	if ok, err := c.installSnapshot(ctx, ls, inbox); err != nil || !ok {
		t.Fatalf("install: %v %v", ok, err)
	}
	if _, err := c.endRebuild(ctx, ls); err != nil {
		t.Fatal(err)
	}
	p, ok, err := c.ReadPage(ctx, 8, 0, 2)
	if err != nil || !ok {
		t.Fatalf("first: %v %v", ok, err)
	}
	if p.Epoch == "" || p.Revision != 12 || p.UnreadCount != 2 || p.Next != 104 || len(p.Items) != 2 || p.Items[0].ID != 108 || p.Items[1].ID != 104 {
		t.Fatalf("first page: %+v", p)
	}
	p, ok, err = c.ReadPage(ctx, 8, p.Next, 2)
	if err != nil || !ok || p.Next != 0 || len(p.Items) != 1 || p.Items[0].ID != 101 {
		t.Fatalf("last page: %+v %v %v", p, ok, err)
	}
	p, ok, err = c.ReadPage(ctx, 8, 101, 2)
	if err != nil || !ok || p.Next != 0 || p.Items == nil || len(p.Items) != 0 {
		t.Fatalf("empty page: %+v %v %v", p, ok, err)
	}
	if err := c.client.HSet(ctx, notificationCacheKeys(8).items, "108", "bad JSON").Err(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := c.ReadPage(ctx, 8, 0, 2); err == nil || ok {
		t.Fatalf("bad JSON accepted: %v %v", ok, err)
	}
}
