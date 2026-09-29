package auth

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRefreshSessionLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	store := NewRefreshStore(client)
	ctx := context.Background()
	raw, err := store.Issue(ctx, 12)
	if err != nil {
		t.Fatal(err)
	}
	if !validRefreshToken(raw) || server.Exists(raw) || !server.Exists(refreshKey(raw)) {
		t.Fatal("invalid token storage")
	}
	if server.TTL(refreshKey(raw)) != RefreshTTL {
		t.Fatal("missing TTL")
	}
	if _, err := store.Rotate(ctx, raw, 13); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatalf("wrong user accepted: %v", err)
	}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Rotate(ctx, raw, 12)
			if err == nil {
				winners.Add(1)
			} else if !errors.Is(err, ErrInvalidRefresh) {
				t.Errorf("rotate: %v", err)
			}
		}()
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("winners = %d", winners.Load())
	}
	if _, err := store.Lookup(ctx, raw); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatal("old token accepted")
	}
	raw, err = store.Issue(ctx, 12)
	if err != nil {
		t.Fatal(err)
	}
	// A server-side write failure must preserve the previous session.
	server.SetError("ERR simulated write failure")
	if _, err := store.Replace(ctx, raw, 13); err == nil {
		t.Fatal("expected failure")
	}
	server.SetError("")
	if id, err := store.Lookup(ctx, raw); err != nil || id != 12 {
		t.Fatalf("old session lost: %v", err)
	}
	next, err := store.Replace(ctx, raw, 13)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Lookup(ctx, raw); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatal("replacement left old session")
	}
	if id, err := store.Lookup(ctx, next); err != nil || id != 13 {
		t.Fatal("replacement has wrong user")
	}
	for i := 0; i < 2; i++ {
		if err := store.Revoke(ctx, next); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Lookup(ctx, next); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatal("revoked token accepted")
	}
	raw, err = store.Issue(ctx, 12)
	if err != nil {
		t.Fatal(err)
	}
	server.FastForward(RefreshTTL + time.Second)
	if _, err := store.Lookup(ctx, raw); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatal("expired token accepted")
	}
}
