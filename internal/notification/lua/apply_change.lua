
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

-- 如果发现上一次写入的未完成标记，就要返回
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

-- 基础完整性检查
if redis.call("HLEN", KEYS[2]) ~= redis.call("ZCARD", KEYS[3]) then
    return redis.error_reply("incomplete notification cache")
end

-- 2. 检查版本。
local incoming = ARGV[1]

-- 检查字符串长度防止精度丢失
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
--先留下applying标记（未完成）
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
