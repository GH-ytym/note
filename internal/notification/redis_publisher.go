package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"note/internal/model"

	"github.com/redis/go-redis/v9"
)

type RedisPublisher struct {
	client *redis.Client
}

func NewRedisPublisher(client *redis.Client) *RedisPublisher {
	return &RedisPublisher{client: client}
}

// redis脚本有个坑：脚本中途报错，前面已经写进去的东西不会撤销
//
//	   新通知
//	     │
//	     ▼
//	┌─────────┐
//	│ Publish │  （Lua 脚本）
//	└────┬────┘
//	     │
//	┌────┴─────────────────────────────┐
//	│                                  │
//	▼                                  ▼
//
// XADD 到 history                  PUBLISH 到 live
// （存下来）                       （实时广播）
//
//	│                                  │
//	│                                  ▼
//	│                            在线的用户 → 立即收到
//	│
//	▼
//
// 断线重连的用户 → XRANGE 拉取错过的history

// history 是 Stream：每条事件有独立的条目 ID，内部保存 outbox_id、name、data 字段。
// sent 是 Hash：以 Outbox ID 为 field、Stream ID 为 value，避免重试时重复写入历史。
// live 是 Pub/Sub 频道：PUBLISH 实时广播，不保存历史，也不记录各窗口接收到了哪里。
var publishNoticeScript = redis.NewScript(publishNoticeLua)

func (p *RedisPublisher) Publish(
	ctx context.Context,
	task model.Outbox,
) (string, error) {
	if task.ID == 0 || task.ReceiverID == 0 || task.Name == "" ||
		!json.Valid([]byte(task.Data)) {
		return "", fmt.Errorf("invalid outbox task %d", task.ID)
	}

	prefix := fmt.Sprintf(
		"note:notifications:{%d}",
		task.ReceiverID,
	) //用{}包裹id，这样遇到redis cluster的时候可以保证被分到同一个槽位不会乱

	//组装lua脚本，保证不同操作的原子性
	streamID, err := publishNoticeScript.Run(
		ctx,
		p.client,
		[]string{
			prefix + ":history", // KEYS[1]
			prefix + ":sent",    // KEYS[2]
			prefix + ":live",    // KEYS[3]
		},
		strconv.FormatUint(uint64(task.ID), 10), // ARGV[1]
		task.Name,                               // ARGV[2]
		task.Data,                               // ARGV[3]
		strconv.FormatUint(uint64(task.ReceiverID), 10), // ARGV[4]
	).Text()

	if err != nil {
		return "", fmt.Errorf(
			"publish notification to Redis: %w",
			err,
		)
	}

	return streamID, nil
}
