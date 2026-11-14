package notification

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisCache 操作每个用户的 v2 通知缓存和实时广播，不直接访问数据库。
// 需要数据库快照的恢复工作由 Rebuilder 负责。
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache 复用已有 Redis 客户端；构造时不读取或初始化缓存。
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// cacheKeys 保存同一用户的 Redis key 名和广播频道名，不保存实际通知数据。
// state 是元数据 Hash；items 是通知 ID 到 Card JSON 的 Hash；order 是按 ID 排序的 ZSet；
type cacheKeys struct {
	state string //状态
	items string //通知
	order string //顺序
	live  string //频道
}

func notificationCacheKeys(userID uint) cacheKeys {
	prefix := fmt.Sprintf("note:notifications:v2:{%d}", userID)

	return cacheKeys{
		state: prefix + ":state",
		items: prefix + ":items",
		order: prefix + ":order",
		live:  prefix + ":live",
	}
}
