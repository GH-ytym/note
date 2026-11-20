package calendarsync

import (
	"context"
	"encoding/json"
	"fmt"

	"note/internal/model"

	"github.com/redis/go-redis/v9"
)

// Delivery 将日历变化发布到接收者的 Redis 频道。
type Delivery struct {
	client *redis.Client
}

func NewDelivery(client *redis.Client) *Delivery {
	return &Delivery{
		client: client,
	}
}

// Channel 返回某个用户接收日历变化的频道。
func Channel(uid uint) string {
	return fmt.Sprintf("note:calendar:{%d}:live", uid)
}

// Deliver 发布一条日历变化。
// 返回 true 代表 Redis 接受了发布，不代表客户端已经收到。
// 重试和 Outbox 完成标记由 Worker 负责。
func (d *Delivery) Deliver(
	ctx context.Context,
	task model.Outbox,
) (bool, error) {
	if task.Name != "cldr" {
		return false, fmt.Errorf(
			"unsupported calendar event: %q",
			task.Name,
		)
	}

	// 与现有 Hub 使用相同的消息格式。
	msg := struct {
		Name string `json:"name"`
		Data string `json:"data"`
	}{
		Name: task.Name,
		Data: task.Data,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return false, fmt.Errorf("encode calendar event: %w", err)
	}

	err = d.client.Publish(
		ctx,
		Channel(task.ReceiverID),
		data,
	).Err()
	if err != nil {
		return false, fmt.Errorf("publish calendar event: %w", err)
	}

	return true, nil
}
