package notification

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

type cacheKeys struct {
	state   string //状态
	items   string //通知
	order   string //顺序
	history string //历史
	live    string //频道
}

func notificationCacheKeys(userID uint) cacheKeys {
	prefix := fmt.Sprintf("note:notifications:v2:{%d}", userID)

	return cacheKeys{
		state:   prefix + ":state",
		items:   prefix + ":items",
		order:   prefix + ":order",
		history: prefix + ":history",
		live:    prefix + ":live",
	}
}
