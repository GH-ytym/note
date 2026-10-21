package notification

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newLeaseCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	m := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: m.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = c.Close() })
	return NewRedisCache(c), m
}

func TestRebuildLease(t *testing.T) {
	ctx := context.Background()

	t.Run("missing state is observed without creating it", func(t *testing.T) {
		c, m := newLeaseCache(t)
		l, err := c.beginRebuild(ctx, 7)
		if err != nil || l == nil {
			t.Fatalf("acquire: lease=%v err=%v", l, err)
		}
		if l.exist || len(l.old) != 0 || m.Exists(notificationCacheKeys(7).state) {
			t.Fatal("missing state was misreported or created")
		}
	})

	t.Run("captures all state fields without changing state", func(t *testing.T) {
		c, _ := newLeaseCache(t)
		key := notificationCacheKeys(7).state
		want := map[string]string{"revision": "10", "unread_count": "3", "applying_revision": "11", "epoch": "old-epoch"}
		for k, v := range want {
			if err := c.client.HSet(ctx, key, k, v).Err(); err != nil {
				t.Fatal(err)
			}
		}
		l, err := c.beginRebuild(ctx, 7)
		if err != nil || l == nil {
			t.Fatalf("acquire: lease=%v err=%v", l, err)
		}
		if !l.exist || len(l.old) != len(want) {
			t.Fatalf("unexpected state: %+v", l)
		}
		got, err := c.client.HGetAll(ctx, key).Result()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatal("state was changed")
		}
		for k, v := range want {
			if l.old[k] != v || got[k] != v {
				t.Fatalf("field %s differs", k)
			}
		}
		if err := c.client.HSet(ctx, key, "revision", "12").Err(); err != nil {
			t.Fatal(err)
		}
		if l.old["revision"] != "10" {
			t.Fatal("old state changed after observation")
		}
	})

	t.Run("wrong state type fails before acquiring lease", func(t *testing.T) {
		c, m := newLeaseCache(t)
		m.Set(notificationCacheKeys(7).state, "invalid")
		l, err := c.beginRebuild(ctx, 7)
		if err == nil || l != nil {
			t.Fatalf("unexpected result: lease=%v err=%v", l, err)
		}
		if m.Exists(rebuildKey(7)) {
			t.Fatal("failed observation left a lease")
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		c, _ := newLeaseCache(t)
		if l, err := c.beginRebuild(ctx, 0); err == nil || l != nil {
			t.Fatalf("zero user: lease=%v err=%v", l, err)
		}
		for _, l := range []*lease{nil, {}, {userID: 1}, {token: "x"}} {
			if ok, err := c.endRebuild(ctx, l); err == nil || ok {
				t.Fatalf("invalid lease: ok=%v err=%v", ok, err)
			}
		}
	})

	t.Run("concurrent callers have one winner", func(t *testing.T) {
		c, _ := newLeaseCache(t)
		const n = 16
		start := make(chan struct{})
		got := make(chan *lease, n)
		errs := make(chan error, n)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				l, err := c.beginRebuild(ctx, 1)
				got <- l
				errs <- err
			}()
		}
		close(start)
		wg.Wait()
		close(got)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		winners := 0
		for l := range got {
			if l != nil {
				winners++
			}
		}
		if winners != 1 {
			t.Fatalf("winners=%d, want 1", winners)
		}
	})

	t.Run("users are independent", func(t *testing.T) {
		c, _ := newLeaseCache(t)
		for _, uid := range []uint{1, 2} {
			if l, err := c.beginRebuild(ctx, uid); err != nil || l == nil {
				t.Fatalf("user %d: lease=%v err=%v", uid, l, err)
			}
		}
	})

	t.Run("release and reacquire", func(t *testing.T) {
		c, _ := newLeaseCache(t)
		a, err := c.beginRebuild(ctx, 1)
		if err != nil || a == nil {
			t.Fatalf("acquire: lease=%v err=%v", a, err)
		}
		if ok, err := c.endRebuild(ctx, a); err != nil || !ok {
			t.Fatalf("release: ok=%v err=%v", ok, err)
		}
		if ok, err := c.endRebuild(ctx, a); err != nil || ok {
			t.Fatalf("repeat release: ok=%v err=%v", ok, err)
		}
		b, err := c.beginRebuild(ctx, 1)
		if err != nil || b == nil {
			t.Fatalf("reacquire: lease=%v err=%v", b, err)
		}
		if ok, err := c.endRebuild(ctx, a); err != nil || ok {
			t.Fatalf("old owner release: ok=%v err=%v", ok, err)
		}
		if ok, err := c.endRebuild(ctx, b); err != nil || !ok {
			t.Fatalf("new owner release: ok=%v err=%v", ok, err)
		}
	})

	t.Run("expired owner cannot release new lease", func(t *testing.T) {
		c, m := newLeaseCache(t)
		a, err := c.beginRebuild(ctx, 1)
		if err != nil || a == nil {
			t.Fatalf("acquire: lease=%v err=%v", a, err)
		}
		if ttl := m.TTL(rebuildKey(1)); ttl != rebuildTTL {
			t.Fatalf("ttl=%v, want %v", ttl, rebuildTTL)
		}
		m.FastForward(rebuildTTL + time.Second)
		b, err := c.beginRebuild(ctx, 1)
		if err != nil || b == nil {
			t.Fatalf("reacquire: lease=%v err=%v", b, err)
		}
		if ok, err := c.endRebuild(ctx, a); err != nil || ok {
			t.Fatalf("expired owner release: ok=%v err=%v", ok, err)
		}
		token, err := c.client.Get(ctx, rebuildKey(1)).Result()
		if err != nil || token != b.token {
			t.Fatalf("new lease was lost: token=%q err=%v", token, err)
		}
		if ok, err := c.endRebuild(ctx, b); err != nil || !ok {
			t.Fatalf("new owner release: ok=%v err=%v", ok, err)
		}
	})

	t.Run("canceled request returns error not busy", func(t *testing.T) {
		c, _ := newLeaseCache(t)
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		if l, err := c.beginRebuild(ctx, 1); err == nil || l != nil {
			t.Fatalf("canceled acquire: lease=%v err=%v", l, err)
		}
	})
}
