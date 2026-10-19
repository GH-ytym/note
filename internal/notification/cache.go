package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

type cacheKeys struct {
	state   string
	items   string
	order   string
	history string
}

type CacheState struct {
	Revision    uint64
	UnreadCount int64
}

func notificationCacheKeys(userID uint) cacheKeys {
	prefix := fmt.Sprintf("note:notifications:v2:{%d}", userID)

	return cacheKeys{
		state:   prefix + ":state",
		items:   prefix + ":items",
		order:   prefix + ":order",
		history: prefix + ":history",
	}
}

var script0 = redis.NewScript(`
-- KEYS：
-- 1 = state
-- 2 = items
-- 3 = order
-- 4 = history
--
-- ARGV：
-- 1 = revision
-- 2 = unread_count
-- 后面每两个参数是一组：通知ID、Card JSON

-- 首次初始化才写入
-- 已有任何相关数据，都交给后续的更新或重建流程处理
if redis.call("EXISTS", KEYS[1], KEYS[2], KEYS[3], KEYS[4]) > 0 then
    return 0
end

for i = 3, #ARGV, 2 do
    local id = ARGV[i]
    local card = ARGV[i + 1]

    redis.call("HSET", KEYS[2], id, card)
    redis.call("ZADD", KEYS[3], id, id)
end

redis.call("HSET", KEYS[1],
    "revision", ARGV[1],
    "unread_count", ARGV[2]
)

return 1
`)

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
