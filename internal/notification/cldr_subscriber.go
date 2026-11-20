package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"note/internal/calendarsync"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func SubscribeCldr(
	ctx context.Context,
	client *redis.Client,
) (*redis.PubSub, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	//开个cldr的订阅
	sub := client.PSubscribe(ctx, "note:calendar:*:live")

	if _, err := sub.ReceiveTimeout(ctx, 3*time.Second); err != nil {
		_ = sub.Close()
		return nil, err
	}
	return sub, nil
}

// 拿出接收者的id
func cldrReceiver(channel string) (uint, error) {
	const (
		prefix = "note:calendar:{"
		suffix = "}:live"
	)
	if !strings.HasPrefix(channel, prefix) ||
		!strings.HasSuffix(channel, suffix) {
		return 0, fmt.Errorf("invalid calendar channel: %q", channel)
	}

	//掐头去尾
	text := strings.TrimSuffix(
		strings.TrimPrefix(channel, prefix),
		suffix,
	)

	id, err := strconv.ParseUint(text, 10, strconv.IntSize)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid calendar receiver: %q", text)
	}

	uid := uint(id)
	//查一下加进去能不能对的上
	if channel != calendarsync.Channel(uid) {
		return 0, fmt.Errorf("noncanonical calendar channel: %q", channel)
	}

	return uid, nil
}

// 转交连接
func RunCldrSubscriber(
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

		case msg, ok := <-messages:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("calendar subscription closed")
			}

			//来自的消息频道
			uid, err := cldrReceiver(msg.Channel)
			if err != nil {
				log.Printf("calendar channel: %v", err)
				//接着处理下一条
				continue
			}

			var event Event
			if err := json.Unmarshal(
				//payload是消息正文
				[]byte(msg.Payload),
				//装载到event里面
				&event,
			); err != nil {
				log.Printf("decode calendar event: %v", err)
				continue
			}

			//前面规定了是cldr
			if event.Name != "cldr" {
				log.Printf("unknown calendar event: %q", event.Name)
				continue
			}

			var change calendarsync.Change
			if err := json.Unmarshal(
				[]byte(event.Data),
				&change,
			); err != nil {
				log.Printf("decode calendar change: %v", err)
				continue
			}

			if change.GroupID == 0 || change.ID == 0 ||
				(change.Kind != "todo" && change.Kind != "event") ||
				(change.Action != "upsert" && change.Action != "remove") {
				log.Printf("invalid calendar change")
				continue
			}

			hub.Send(uid, event)
		}
	}
}
