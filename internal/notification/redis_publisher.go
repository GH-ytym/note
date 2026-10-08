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
var publishNoticeScript = redis.NewScript(`
-- 写入前检查历史 key 的类型。
local historyType = redis.call("TYPE", KEYS[1]).ok
if historyType ~= "none" and historyType ~= "stream" then
    return redis.error_reply("notification history must be a stream")
end

-- 查询这个 Outbox 任务是否已经写过历史；sent 不是用户已收到／已读的回执。
local streamID = redis.call("HGET", KEYS[2], ARGV[1])

if not streamID then
    -- 没有写过：新增历史，由 Redis 生成事件 ID。
    -- XADD表示往历史stream追加一条记录，"*"表示自动生成id
    --streamID就是这次生成的id
    streamID = redis.call("XADD", KEYS[1], "*",
        "outbox_id", ARGV[1],
        "name", ARGV[2],
        "data", ARGV[3])

    -- 保存 Outbox ID → 事件 ID 的对应关系。
    redis.call("HSET", KEYS[2], ARGV[1], streamID)
    --对应line57
end

-- 广播完整事件；重试时仍使用同一个事件 ID。
local message = cjson.encode({
    receiver_id = tonumber(ARGV[4]),
    event = {
        id = streamID,
        name = ARGV[2],
        data = ARGV[3]
    }
})

--这里是publish note:notifications:{xxx}:live message
redis.call("PUBLISH", KEYS[3], message)
return streamID
`)

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
