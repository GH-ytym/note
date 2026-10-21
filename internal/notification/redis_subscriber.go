package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// 建立新频道的订阅并等待 Redis 确认
func SubscribeRedis(
	ctx context.Context,
	client *redis.Client,
) (*redis.PubSub, error) {
	subscribeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	sub := client.PSubscribe(
		subscribeCtx,
		"note:notifications:v2:*:live",
	)

	if _, err := sub.ReceiveTimeout(
		subscribeCtx,
		3*time.Second,
	); err != nil {
		_ = sub.Close()
		return nil, fmt.Errorf("subscribe notifications: %w", err)
	}

	return sub, nil
}

// 从频道名取出user
func receiverFromChannel(channel string) (uint, error) {
	const prefix = "note:notifications:v2:{"
	const suffix = "}:live"

	if !strings.HasPrefix(channel, prefix) ||
		!strings.HasSuffix(channel, suffix) {
		return 0, fmt.Errorf("invalid notification channel")
	}

	text := strings.TrimSuffix(
		strings.TrimPrefix(channel, prefix),
		suffix,
	)

	id, err := strconv.ParseUint(text, 10, strconv.IntSize)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid notification receiver")
	}

	userID := uint(id)

	// 只接受 key 生成函数产生的标准格式
	if channel != notificationCacheKeys(userID).live {
		return 0, fmt.Errorf("noncanonical notification channel")
	}

	return userID, nil
}

// 持续接收新广播，转交给对应用户的 Hub 连接。
func RunRedisSubscriber(
	ctx context.Context,
	sub *redis.PubSub,
	hub *Hub,
) error {
	defer sub.Close()

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
				return fmt.Errorf("notification subscription closed")
			}

			// 1. 从频道确定接收者。
			userID, err := receiverFromChannel(message.Channel)
			if err != nil {
				log.Printf("invalid notification channel: %v", err)
				continue
			}

			// 2. 解析 Lua 广播的外层 Event。
			var event Event
			if err := json.Unmarshal(
				[]byte(message.Payload),
				&event,
			); err != nil {
				log.Printf("decode notification event: %v", err)
				continue
			}

			if event.ID == "" ||
				event.Name != "notification.change" {
				log.Printf("invalid notification event")
				continue
			}

			// 3. 检查里面确实是有效的 Change。
			var change Change
			if err := json.Unmarshal(
				[]byte(event.Data),
				&change,
			); err != nil {
				log.Printf("decode notification change: %v", err)
				continue
			}

			if change.Revision == 0 ||
				change.Revision > uint64(1<<63-1) ||
				change.UnreadCount < 0 ||
				change.Upserts == nil ||
				change.RemovedIDs == nil {
				log.Printf("invalid notification change")
				continue
			}

			// 4. 原样交给这个用户的各个在线连接。
			hub.Send(userID, event)
		}
	}
}
