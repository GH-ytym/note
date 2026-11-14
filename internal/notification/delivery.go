package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"note/internal/model"
)

// SnapshotLoader 提供一个用户的完整数据库快照，供 Rebuilder 使用。
// 只依赖投递所需的方法，方便测试。
type SnapshotLoader interface {
	LoadSnapshot(ctx context.Context, userID uint) (Inbox, error)
}

// ChangeCache 是 Delivery 所需的增量缓存写入能力。
type ChangeCache interface {
	ApplyChange(
		ctx context.Context,
		userID uint,
		change Change,
	) (ApplyResult, error)
}

// CacheRebuilder 提供单次重建能力；返回已安装版本、安装成功标志和执行错误。
type CacheRebuilder interface {
	Rebuild(
		ctx context.Context,
		uid uint,
	) (uint64, bool, error)
}

// Delivery 处理单条 Outbox，协调增量应用与必要的重建。
// 重试调度和 Outbox 完成标记由外层 Worker 负责。
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

// Deliver 返回当前缓存是否已经覆盖这条任务对应的业务版本。
// true 允许 Worker 标记任务完成，不表示各个浏览器都已经收到消息。
// false, nil 表示本次恢复未完成；error 表示执行失败。两者都不能标记任务完成。
//
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
