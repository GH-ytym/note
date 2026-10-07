package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// 对应 Lua 中广播的外层 JSON。
// Event.Data 仍然是一张 Card 的 JSON 字符串。
type redisNotice struct {
	ReceiverID uint  `json:"receiver_id"`
	Event      Event `json:"event"`
}

// 建立 Redis 订阅，并等待 Redis 确认订阅成功。
func SubscribeRedis(
	ctx context.Context,
	client *redis.Client,
) (*redis.PubSub, error) {
	subscribeCtx, cancel := context.WithTimeout(
		ctx,
		3*time.Second,
	)
	defer cancel()

	// * 匹配不同接收者的频道。
	// 例如 note:notifications:{8}:live。
	sub := client.PSubscribe(
		subscribeCtx,
		"note:notifications:*:live",
	)

	// PSubscribe 返回后，还要等待 Redis 的订阅确认。
	if _, err := sub.ReceiveTimeout(
		subscribeCtx,
		3*time.Second,
	); err != nil {
		_ = sub.Close()
		return nil, fmt.Errorf(
			"subscribe Redis notifications: %w",
			err,
		)
	}

	return sub, nil
}

// 持续接收 Redis 广播，再转交给本后端的 Hub。
func RunRedisSubscriber(
	ctx context.Context,
	sub *redis.PubSub,
	hub *Hub,
) error {
	defer sub.Close()

	// go-redis 将收到的广播放进这个 Go channel。
	messages := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			return nil

		case message, ok := <-messages:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf(
					"Redis notification subscription closed",
				)
			}

			var notice redisNotice
			if err := json.Unmarshal(
				[]byte(message.Payload),
				&notice,
			); err != nil {
				log.Printf(
					"decode Redis notification: %v",
					err,
				)
				continue
			}

			if notice.ReceiverID == 0 ||
				notice.Event.ID == "" ||
				notice.Event.Name == "" ||
				!json.Valid([]byte(notice.Event.Data)) {
				log.Printf("invalid Redis notification")
				continue
			}

			// 频道中的用户必须和消息中的接收者一致。
			expectedChannel := fmt.Sprintf(
				"note:notifications:{%d}:live",
				notice.ReceiverID,
			)
			if message.Channel != expectedChannel {
				log.Printf(
					"notification receiver does not match channel",
				)
				continue
			}

			hub.Send(notice.ReceiverID, notice.Event)
		}
	}
}
