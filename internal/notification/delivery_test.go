package notification

import (
	"context"
	"errors"
	"testing"

	"note/internal/model"
)

type applyFunc func(context.Context, uint, Change) (ApplyResult, error)

func (f applyFunc) ApplyChange(ctx context.Context, uid uint, c Change) (ApplyResult, error) {
	return f(ctx, uid, c)
}

type rebuildFunc func(context.Context, uint) (uint64, bool, error)

func (f rebuildFunc) Rebuild(ctx context.Context, uid uint) (uint64, bool, error) {
	return f(ctx, uid)
}

func TestDeliveryCompletion(t *testing.T) {
	failure := errors.New("injected failure")
	task := model.Outbox{ID: 1, ReceiverID: 42, Revision: 9, Name: "notification.change",
		Data: `{"revision":"9","upserts":[],"removed_ids":[],"unread_count":0}`}
	for _, tc := range []struct {
		name                         string
		result                       ApplyResult
		applyErr                     error
		rev                          uint64
		installed                    bool
		rebuildErr                   error
		wantOK, wantErr, wantRebuild bool
	}{
		{name: "applied", result: ApplyApplied, wantOK: true},
		{name: "ignored", result: ApplyIgnored, wantOK: true},
		{name: "missing", result: ApplyMissing, rev: 9, installed: true, wantOK: true, wantRebuild: true},
		{name: "gap covered by newer snapshot", result: ApplyGap, rev: 12, installed: true, wantOK: true, wantRebuild: true},
		{name: "damaged", result: ApplyRebuild, rev: 9, installed: true, wantOK: true, wantRebuild: true},
		{name: "busy or conflict", result: ApplyMissing, wantRebuild: true},
		{name: "snapshot behind task", result: ApplyGap, rev: 8, installed: true, wantErr: true, wantRebuild: true},
		{name: "apply error", applyErr: failure, wantErr: true},
		{name: "rebuild error", result: ApplyRebuild, rebuildErr: failure, wantErr: true, wantRebuild: true},
		{name: "unknown result", result: "unknown", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			cache := applyFunc(func(_ context.Context, uid uint, c Change) (ApplyResult, error) {
				if uid != task.ReceiverID || c.Revision != task.Revision {
					t.Fatalf("wrong apply arguments: %d %+v", uid, c)
				}
				return tc.result, tc.applyErr
			})
			r := rebuildFunc(func(_ context.Context, uid uint) (uint64, bool, error) {
				calls++
				if uid != task.ReceiverID {
					t.Fatalf("wrong rebuild user: %d", uid)
				}
				return tc.rev, tc.installed, tc.rebuildErr
			})
			ok, err := NewDelivery(cache, r).Deliver(context.Background(), task)
			if ok != tc.wantOK || (err != nil) != tc.wantErr {
				t.Fatalf("ok=%v err=%v", ok, err)
			}
			wantCalls := 0
			if tc.wantRebuild {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("rebuild calls=%d want=%d", calls, wantCalls)
			}
			if (tc.applyErr != nil || tc.rebuildErr != nil) && !errors.Is(err, failure) {
				t.Fatalf("lost cause: %v", err)
			}
		})
	}
}

func TestDeliveryRejectsMismatchedRevisionBeforeRedis(t *testing.T) {
	cache := applyFunc(func(context.Context, uint, Change) (ApplyResult, error) {
		t.Fatal("invalid task reached Redis")
		return ApplyApplied, nil
	})
	r := rebuildFunc(func(context.Context, uint) (uint64, bool, error) {
		t.Fatal("invalid task triggered rebuild")
		return 9, true, nil
	})
	ok, err := NewDelivery(cache, r).Deliver(context.Background(), model.Outbox{
		ID: 1, ReceiverID: 42, Revision: 9, Name: "notification.change",
		Data: `{"revision":"8","upserts":[],"removed_ids":[],"unread_count":0}`,
	})
	if ok || err == nil {
		t.Fatalf("mismatched task accepted: %v %v", ok, err)
	}
}
