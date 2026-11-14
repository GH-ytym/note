package notification

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Rebuilder 协调重建资格、数据库完整快照和 Redis 安装，不负责 Outbox 完成标记。
type Rebuilder struct {
	repo  SnapshotLoader
	cache *RedisCache
}

func NewRebuilder(
	repo SnapshotLoader,
	cache *RedisCache,
) *Rebuilder {
	return &Rebuilder{
		repo:  repo,
		cache: cache,
	}
}

// Rebuild 尝试一次完整缓存重建，不在内部循环重试。
// 返回值依次是已安装的业务版本、是否安装成功、执行错误。
// 资格忙、过期或状态冲突时返回 0, false, nil；只有成功才返回快照版本。
// 取得资格后，无论读取或安装是否成功，都会尝试释放资格。
//
// beginRebuild
//
//	↓ 取得资格后
//
// LoadSnapshot
//
//	↓ 读取数据库快照后
//
// installSnapshot
//
//	↓ 无论成功还是失败
//
// endRebuild
func (r *Rebuilder) Rebuild(ctx context.Context, uid uint,
) (uint64, bool, error) {
	//取得重建资格
	ls, err := r.cache.beginRebuild(ctx, uid)

	if err != nil {
		return 0, false, fmt.Errorf("begin cache rebuild: %w", err)
	}
	if ls == nil {
		return 0, false, nil
	}

	//总之要记得释放资格
	defer func() {
		//使用独立的短超时
		ctx1, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		if _, err := r.cache.endRebuild(ctx1, ls); err != nil {
			log.Printf(
				"release notification rebuild lease for user %d: %v",
				uid,
				err,
			)
		}
	}()

	//取得资格后拿数据库的完整快照
	inbox, err := r.repo.LoadSnapshot(ctx, uid)
	if err != nil {
		return 0, false, fmt.Errorf("load rebuild snapshot: %w", err)
	}

	//检查资格和旧状态，再决定是否写入
	ok, err := r.cache.installSnapshot(ctx, ls, inbox)
	if err != nil {
		return 0, false, fmt.Errorf("install rebuild snapshot: %w", err)
	}
	if !ok {
		return 0, false, nil
	}

	// 只有安装成功才返回被安装的版本
	return inbox.Revision, true, nil
}
