package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type ApplyResult string

const (
	ApplyApplied ApplyResult = "applied" // 已更新
	ApplyIgnored ApplyResult = "ignored" // 重复或旧消息
	ApplyMissing ApplyResult = "missing" // 缓存不存在
	ApplyGap     ApplyResult = "gap"     // 中间缺少版本
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

var script0 = redis.NewScript(`
-- KEYS：
-- 1 = state
-- 2 = items
-- 3 = order
-- 4 = history
--
-- ARGV：
-- 1 = revision
-- 2 = unread_count
-- 后面每两个参数是一组：通知ID、Card JSON

-- 首次初始化才写入
-- 已有任何相关数据，都交给后续的更新或重建流程处理
if redis.call("EXISTS", KEYS[1], KEYS[2], KEYS[3], KEYS[4]) > 0 then
    return 0
end

for i = 3, #ARGV, 2 do
    local id = ARGV[i]
    local card = ARGV[i + 1]

    redis.call("HSET", KEYS[2], id, card)
    redis.call("ZADD", KEYS[3], id, id)
end

redis.call("HSET", KEYS[1],
    "revision", ARGV[1],
    "unread_count", ARGV[2]
)

return 1
`)

var applyChangeScript = redis.NewScript(`
-- KEYS：
-- 1=state
-- 2=items
-- 3=order
-- 4=history
-- 5=live
--
-- ARGV：
-- 1=新版本
-- 2=预期旧版本
-- 3=最新未读数
-- 4=upserts 数量
-- 5=完整 Change JSON
-- 从6开始：ID、Card JSON 成对排列，随后是待删除 ID

local function keyType(key)
    return redis.call("TYPE", key).ok
end

-- 1. 检查缓存是否存在、类型是否正确。
local stateType = keyType(KEYS[1])
if stateType == "none" then
    return "missing"
end
if stateType ~= "hash" then
    return redis.error_reply("invalid state key type")
end

local itemsType = keyType(KEYS[2])
local orderType = keyType(KEYS[3])
local historyType = keyType(KEYS[4])

if itemsType ~= "none" and itemsType ~= "hash" then
    return redis.error_reply("invalid items key type")
end
if orderType ~= "none" and orderType ~= "zset" then
    return redis.error_reply("invalid order key type")
end
if historyType ~= "none" and historyType ~= "stream" then
    return redis.error_reply("invalid history key type")
end

-- 上一次应用未完成，不能把它当作成功或直接继续。
if redis.call("HEXISTS", KEYS[1], "applying_revision") == 1 then
    return redis.error_reply("notification cache needs rebuild")
end

local current = redis.call("HGET", KEYS[1], "revision")
local unread = redis.call("HGET", KEYS[1], "unread_count")

if not current or not unread then
    return redis.error_reply("incomplete cache state")
end

local function validInteger(value)
    return value == "0" or string.match(value, "^[1-9][0-9]*$") ~= nil
end

if not validInteger(current) or not validInteger(unread) then
    return redis.error_reply("invalid cache state")
end

-- 基础完整性检查，不等同于完整的数据校验。
if redis.call("HLEN", KEYS[2]) ~= redis.call("ZCARD", KEYS[3]) then
    return redis.error_reply("incomplete notification cache")
end

-- 2. 检查版本。
local incoming = ARGV[1]

-- 不把版本转为浮点数，避免大整数精度丢失。
if #incoming < #current or
   (#incoming == #current and incoming <= current) then
    return "ignored"
end

if current ~= ARGV[2] then
    return "gap"
end

local upsertCount = tonumber(ARGV[4])
local position = 6

-- 3. 标记开始应用。
-- Lua 报错不会自动回滚；失败后此标记留给恢复流程处理。
redis.call("HSET", KEYS[1], "applying_revision", incoming)

-- 4. 新增或更新卡片。
for i = 1, upsertCount do
    local id = ARGV[position]
    local card = ARGV[position + 1]

    redis.call("HSET", KEYS[2], id, card)
    redis.call("ZADD", KEYS[3], id, id)

    position = position + 2
end

-- 5. 删除卡片。
for i = position, #ARGV do
    local id = ARGV[i]

    redis.call("HDEL", KEYS[2], id)
    redis.call("ZREM", KEYS[3], id)
end

-- 6. 保存变化历史，暂时保留最近1000条。
local streamID = redis.call("XADD", KEYS[4],
    "MAXLEN", "1000",
    "*",
    "revision", incoming,
    "name", "notification.change",
    "data", ARGV[5]
)

-- 7. 保存最新版本和绝对未读数。
redis.call("HSET", KEYS[1],
    "revision", incoming,
    "unread_count", ARGV[3]
)

-- 8. 广播；data 保持 Go 编码的 JSON 字符串。
local message = cjson.encode({
    id = streamID,
    name = "notification.change",
    data = ARGV[5]
})

redis.call("PUBLISH", KEYS[5], message)

-- 9. 全部完成，清除标记。
redis.call("HDEL", KEYS[1], "applying_revision")

return "applied"
`)

// InitSnapshot 在缓存完全不存在时，安装一份通知快照
// 返回 true 表示本次成功建立缓存；false 表示已有数据但没有写入
// inbox 必须属于 userID，且列表、版本、未读数来自同一个数据库快照,调用方负责满足这个条件
func (r *RedisCache) InitSnapshot(
	ctx context.Context,
	userID uint,
	inbox Inbox,
) (bool, error) {
	if userID == 0 {
		return false, fmt.Errorf("userID cannot be zero")
	}
	if inbox.UnreadCount < 0 {
		return false, fmt.Errorf("unread count cannot be negative")
	}
	if inbox.Revision > uint64(1<<63-1) {
		return false, fmt.Errorf("revision exceeds SQLite integer range")
	}

	keys := notificationCacheKeys(userID)

	// 版本以十进制字符串传入
	//需要转为浮点数
	args := []any{
		strconv.FormatUint(inbox.Revision, 10),
		strconv.FormatInt(inbox.UnreadCount, 10),
	}

	// 快照应来自同一次数据库查询；重复 ID 会由 Redis 的 HSET/ZADD 覆盖。
	for _, card := range inbox.Items {
		// ZSet 分数使用浮点数，整数精确范围限制在这里
		if card.ID == 0 || uint64(card.ID) > uint64(1<<53-1) {
			return false, fmt.Errorf("invalid notification ID: %d", card.ID)
		}
		//单条notification不会出现重复id
		//所以初始化cache的时候不用去重
		data, err := json.Marshal(card)
		if err != nil {
			return false, fmt.Errorf("encode notification card: %w", err)
		}

		args = append(
			args,
			strconv.FormatUint(uint64(card.ID), 10),
			string(data),
		)
	}

	installed, err := script0.Run(
		ctx,
		r.client,
		[]string{
			keys.state,
			keys.items,
			keys.order,
			keys.history,
		},
		args...,
	).Int()
	if err != nil {
		return false, fmt.Errorf("initialize notification cache: %w", err)
	}

	return installed == 1, nil
}

func (r *RedisCache) ApplyChange(
	ctx context.Context,
	userID uint,
	change Change,
) (ApplyResult, error) {
	if userID == 0 {
		return "", fmt.Errorf("userID cannot be zero")
	}
	if change.Revision == 0 || change.Revision > uint64(1<<63-1) {
		return "", fmt.Errorf("invalid change revision")
	}
	if change.UnreadCount < 0 {
		return "", fmt.Errorf("unread count cannot be negative")
	}

	// JSON 中输出[]  免得出现null
	if change.Upserts == nil {
		change.Upserts = []Card{}
	}
	if change.RemovedIDs == nil {
		change.RemovedIDs = []uint{}
	}

	payload, err := json.Marshal(change)
	if err != nil {
		return "", fmt.Errorf("encode notification change: %w", err)
	}

	// 固定参数占前5个位置
	args := []any{
		strconv.FormatUint(change.Revision, 10),
		strconv.FormatUint(change.Revision-1, 10),
		strconv.FormatInt(change.UnreadCount, 10),
		len(change.Upserts),
		string(payload),
	}

	// 每张卡片追加两个参数
	//ID+完整 Card JSON
	for _, card := range change.Upserts {
		// ID 被用作 ZSet 分数，限制在整数可精确表示的范围。
		if card.ID == 0 || uint64(card.ID) > uint64(1<<53-1) {
			return "", fmt.Errorf(
				"invalid notification ID: %d", card.ID,
			)
		}

		data, err := json.Marshal(card)
		if err != nil {
			return "", fmt.Errorf(
				"encode notification card: %w", err,
			)
		}

		args = append(
			args,
			strconv.FormatUint(uint64(card.ID), 10),
			string(data),
		)
	}

	// 删除id
	for _, id := range change.RemovedIDs {
		if id == 0 || uint64(id) > uint64(1<<53-1) {
			return "", fmt.Errorf(
				"invalid removed notification ID: %d", id,
			)
		}

		args = append(args, strconv.FormatUint(uint64(id), 10))
	}

	keys := notificationCacheKeys(userID)

	value, err := applyChangeScript.Run(
		ctx,
		r.client,
		[]string{
			keys.state,
			keys.items,
			keys.order,
			keys.history,
			keys.live,
		},
		args...,
	).Text()
	if err != nil {
		return "", fmt.Errorf("apply notification change: %w", err)
	}

	result := ApplyResult(value)
	switch result {
	case ApplyApplied, ApplyIgnored, ApplyMissing, ApplyGap:
		return result, nil
	default:
		return "", fmt.Errorf("unexpected apply result: %q", value)
	}
}
