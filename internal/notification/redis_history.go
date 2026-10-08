package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func ReadHistoryAfter(
	ctx context.Context,
	client *redis.Client,
	userID uint,
	// 当前窗口最后处理的 Event.ID（Stream ID），由重连请求带回。
	// 它不是通知 ID 或 Outbox ID；sent 不记录窗口的接收进度。
	afterID string,
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
