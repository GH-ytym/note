package notification

import (
	"context"
	"fmt"
	apperrors "note/internal/errors"
)

// Reader 组合 Redis 分页读取与缓存恢复，供列表业务使用。
type Reader struct {
	cache   *RedisCache
	rebuild CacheRebuilder
}

func NewReader(
	cache *RedisCache,
	rebuild CacheRebuilder,
) *Reader {
	return &Reader{
		cache:   cache,
		rebuild: rebuild,
	}
}

// List 按通知 ID 倒序读取一页，before 为 0 表示从最新通知开始，limit 范围为 1 到 100。
// 缓存不可用时只尝试一次重建和一次重读；仍不可用返回 ErrCacheUnavailable。
// Redis 执行错误直接返回，不把错误伪装成空列表。
//
// ReadPage
//
//	├─ 成功 → 返回这一页
//	├─ 执行错误 → 返回错误
//	└─ 缓存需要恢复
//	        ↓
//	     Rebuild 一次
//	        ↓
//	     再 ReadPage 一次
//	        ├─ 成功 → 返回
//	        └─ 仍不可用 → 明确返回错误\
func (r *Reader) List(
	ctx context.Context,
	uid uint,
	before uint,
	limit uint,
) (Page, error) {
	//读缓存
	page, ok, err := r.cache.ReadPage(ctx, uid, before, limit)
	if err != nil {
		return Page{}, err
	}
	//ok就没了
	if ok {
		return page, nil
	}

	//没error但不ok
	//尝试一次重建
	_, _, err = r.rebuild.Rebuild(ctx, uid)
	if err != nil {
		return Page{}, fmt.Errorf("failed to rebuild page %d: %w", uid, err)
	}

	//rebuild只有重建缓存，没拿到分页
	//再拿一次
	page, ok, err = r.cache.ReadPage(ctx, uid, before, limit)
	if err != nil {
		return Page{}, err
	}
	//还是不行就先返回
	if !ok {
		return Page{}, apperrors.ErrCacheUnavailable
	}

	return page, nil
}
