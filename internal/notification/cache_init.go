package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var script0 = redis.NewScript(initSnapshotLua)

// InitSnapshot 在缓存完全不存在时，安装一份通知快照
// 返回 true 表示本次成功建立缓存；false 表示已有数据但没有写入
// inbox 必须属于 userID，且列表、版本、未读数来自同一个数据库快照,调用方负责满足这个条件
func (r *RedisCache) InitSnapshot(
	ctx context.Context,
	userID uint,
	inbox Inbox,
) (bool, error) {
	if userID == 0 {
		return false, fmt.Errorf("userID cannot be zero")
	}
	if inbox.UnreadCount < 0 {
		return false, fmt.Errorf("unread count cannot be negative")
	}
	if inbox.Revision > uint64(1<<63-1) {
		return false, fmt.Errorf("revision exceeds SQLite integer range")
	}

	keys := notificationCacheKeys(userID)

	// 版本以十进制字符串传入
	//需要转为浮点数
	args := []any{
		strconv.FormatUint(inbox.Revision, 10),
		strconv.FormatInt(inbox.UnreadCount, 10),
	}

	// 快照应来自同一次数据库查询；重复 ID 会由 Redis 的 HSET/ZADD 覆盖。
	for _, card := range inbox.Items {
		// ZSet 分数使用浮点数，整数精确范围限制在这里
		if card.ID == 0 || uint64(card.ID) > uint64(1<<53-1) {
			return false, fmt.Errorf("invalid notification ID: %d", card.ID)
		}
		//单条notification不会出现重复id
		//所以初始化cache的时候不用去重
		data, err := json.Marshal(card)
		if err != nil {
			return false, fmt.Errorf("encode notification card: %w", err)
		}

		args = append(
			args,
			strconv.FormatUint(uint64(card.ID), 10),
			string(data),
		)
	}

	installed, err := script0.Run(
		ctx,
		r.client,
		[]string{
			keys.state,
			keys.items,
			keys.order,
			keys.history,
		},
		args...,
	).Int()
	if err != nil {
		return false, fmt.Errorf("initialize notification cache: %w", err)
	}

	return installed == 1, nil
}
