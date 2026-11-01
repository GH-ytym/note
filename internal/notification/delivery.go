package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"note/internal/model"
)

// 只依赖投递所需的方法，方便测试。
type SnapshotLoader interface {
	LoadSnapshot(ctx context.Context, userID uint) (Inbox, error)
}

type ChangeCache interface {
	ApplyChange(
		ctx context.Context,
		userID uint,
		change Change,
	) (ApplyResult, error)
}

type CacheRebuilder interface {
	Rebuild(
		ctx context.Context,
		uid uint,
	) (uint64, bool, error)
}

type Delivery struct {
	cache   ChangeCache
	rebuild CacheRebuilder
}

func NewDelivery(
	cache ChangeCache,
	rebuild CacheRebuilder,
) *Delivery {
	return &Delivery{
		cache:   cache,
		rebuild: rebuild,
	}
}

// 解析 Outbox 中的 Change
//
//	↓
//
// ApplyChange
//
//	├─ applied / ignored → 完成
//	├─ missing / gap / rebuild
//	│      ↓
//	│   Rebuild
//	│      ├─ 未安装成功 → 未完成
//	│      ├─ 快照版本 < 任务版本 → 未完成，报错
//	│      └─ 快照版本 ≥ 任务版本 → 完成
//	└─ 执行出错 → 返回错误
func (d *Delivery) Deliver(
	ctx context.Context,
	task model.Outbox,
) (bool, error) {
	if task.ID == 0 || task.ReceiverID == 0 {
		return false, fmt.Errorf("invalid outbox task %d", task.ID)
	}
	if task.Name != "notification.change" {
		return false, fmt.Errorf(
			"unsupported outbox event: %q", task.Name,
		)
	}
	if task.Revision == 0 || task.Revision > uint64(1<<63-1) {
		return false, fmt.Errorf("invalid outbox revision")
	}

	var change Change
	if err := json.Unmarshal([]byte(task.Data), &change); err != nil {
		return false, fmt.Errorf("decode outbox change: %w", err)
	}

	if change.Revision != task.Revision {
		return false, fmt.Errorf(
			"outbox revision %d differs from change revision %d",
			task.Revision,
			change.Revision,
		)
	}
	if change.UnreadCount < 0 ||
		change.Upserts == nil ||
		change.RemovedIDs == nil {
		return false, fmt.Errorf("invalid outbox change")
	}

	result, err := d.cache.ApplyChange(ctx, task.ReceiverID, change)
	if err != nil {
		return false, fmt.Errorf("apply outbox change: %w", err)
	}

	switch result {
	case ApplyApplied, ApplyIgnored:
		return true, nil

	case ApplyMissing, ApplyGap, ApplyRebuild:
		rev, ok, err := d.rebuild.Rebuild(ctx, task.ReceiverID)
		if err != nil {
			return false, fmt.Errorf("rebuild notification cache: %w", err)
		}
		if !ok {
			return false, nil
		}

		if rev < task.Revision {
			return false, fmt.Errorf(
				"rebuilt revision %d is behind task revision %d",
				rev,
				task.Revision,
			)
		}

		return true, nil

	default:
		return false, fmt.Errorf("unexpected apply result: %q", result)
	}
}
