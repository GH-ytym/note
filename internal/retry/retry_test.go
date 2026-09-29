package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoOptions(t *testing.T) {
	transient := errors.New("retryable")
	fatal := errors.New("fatal")
	for _, tc := range []struct {
		name     string
		failures int
		failure  error
		calls    int
		want     error
	}{
		{"success", 0, nil, 1, nil},
		{"retry then success", 2, transient, 3, nil},
		{"stop immediately", 1, fatal, 1, fatal},
		{"exhausted", 10, transient, 3, transient},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := Do(context.Background(), func() error {
				calls++
				if calls <= tc.failures {
					return tc.failure
				}
				return nil
			}, Options{MaxAttempts: 3, ShouldRetry: func(err error) bool { return errors.Is(err, transient) }})
			if calls != tc.calls || !errors.Is(err, tc.want) {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestDoCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := Do(ctx, func() error { calls++; return nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal(calls, err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Do(ctx, func() error { close(started); return errors.New("retry") }, Options{MaxAttempts: 3, InitialDelay: time.Hour})
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt retry")
	}
}

func TestDoDefaultAndInvalidOptions(t *testing.T) {
	calls := 0
	failure := errors.New("retry")
	err := Do(context.Background(), func() error { calls++; return failure })
	if calls != 3 || !errors.Is(err, failure) {
		t.Fatal(calls, err)
	}
	for _, options := range []Options{{}, {MaxAttempts: 1, InitialDelay: -1}} {
		err := Do(context.Background(), func() error { t.Fatal("must not execute"); return nil }, options)
		if err == nil {
			t.Fatal("invalid options accepted")
		}
	}
}
