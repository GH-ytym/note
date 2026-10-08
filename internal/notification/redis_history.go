package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisHistoryRepository 负责读取推送历史。
// SQLite 的 Repository 负责通知记录，两者保存的内容不同。
type RedisHistoryRepository struct {
	client *redis.Client
}

func NewRedisHistoryRepository(
	client *redis.Client,
) *RedisHistoryRepository {
	return &RedisHistoryRepository{client: client}
}

func (r *RedisHistoryRepository) ReadAfter(
	ctx context.Context,
	userID uint,
	afterID string,
) ([]Event, error) {
	// 只是调用查询函数
	return ReadHistoryAfter(ctx, r.client, userID, afterID)
}

func ReadHistoryAfter(
	ctx context.Context,
	client *redis.Client,
	userID uint,
	afterID string, //这里是前端传过来的，因为publish的时候把streamid传过去了，前端拿到最新id就是这个afterid
	// 对应 Stream 发给这个窗口的 Event.ID；sent 不记录窗口的接收进度。
) ([]Event, error) {
	if userID == 0 || afterID == "" {
		return nil, fmt.Errorf("missing user ID or event ID")
	}

	//拼接key
	key := fmt.Sprintf(
		"note:notifications:{%d}:history",
		userID,
	)

	rows, err := client.XRangeN(
		ctx,
		key,
		"("+afterID, // 从这个 ID 之后开始（不含这个）
		"+",         // 一直查到最新位置
		100,         // 一页最多 100 条
	).Result()
	if err != nil {
		return nil, fmt.Errorf("read notification history: %w", err)
	}

	events := make([]Event, 0, len(rows))
	for _, r := range rows {
		name, nameOK := r.Values["name"].(string)
		data, dataOK := r.Values["data"].(string)
		if !nameOK || !dataOK || name == "" || !json.Valid([]byte(data)) {
			return nil, fmt.Errorf("invalid history event: %s", r.ID)
		}
		events = append(events, Event{
			ID:   r.ID,
			Name: name,
			Data: data,
		})
	}
	return events, nil
}
