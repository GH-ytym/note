-- KEYS：
-- 1=state
-- 2=items
-- 3=order
--
-- ARGV：
-- 1=通知 ID 上界，例如 +inf 或 (104
-- 2=最多读取的 ID 数量，即 limit+1

local function keyType(key)
    return redis.call("TYPE", key).ok
end

local function validInteger(val)
    return val == "0" or
            string.match(val, "^[1-9][0-9]*$") ~= nil
end

-- 1. 检查参数。
local count = tonumber(ARGV[2])

if not count or count < 2 or count > 101 or
        count ~= math.floor(count) then
    return redis.error_reply("invalid page count")
end

-- 2. 检查缓存的 key 类型。
local kind = keyType(KEYS[1])

if kind == "none" then
    return {"missing"}
end

if kind ~= "hash" then
    return redis.error_reply("invalid state key type")
end

local itemsType = keyType(KEYS[2])
local orderType = keyType(KEYS[3])

if itemsType ~= "none" and itemsType ~= "hash" then
    return {"rebuild"}
end

if orderType ~= "none" and orderType ~= "zset" then
    return {"rebuild"}
end

-- 3. 未完成的写入不能作为正常列表返回。
if redis.call("HEXISTS", KEYS[1], "applying_revision") == 1 then
    return {"rebuild"}
end

local epoch = redis.call("HGET", KEYS[1], "epoch")
local rev = redis.call("HGET", KEYS[1], "revision")
local unread = redis.call("HGET", KEYS[1], "unread_count")

if not epoch or epoch == "" or not rev or not unread then
    return {"rebuild"}
end

if not validInteger(rev) or not validInteger(unread) then
    return {"rebuild"}
end

-- 4. 卡片数量和排序成员数量应该一致。
if redis.call("HLEN", KEYS[2]) ~= redis.call("ZCARD", KEYS[3]) then
    return {"rebuild"}
end

-- 5. 按 ID 倒序读取，额外取一条用于判断下一页。
local ids = redis.call(
        "ZREVRANGEBYSCORE",
        KEYS[3],
        ARGV[1],
        "-inf",
        "LIMIT", 0, count
)

local size = #ids
local nextID = "0"

if size == count then
    -- 最后一条只用于判断还有下一页，要把它排除
    size = size - 1
    nextID = ids[size]
end

-- 6. 根据这一页的 ID，读取完整 Card JSON。
local cards = {}

for i = 1, size do
    local card = redis.call("HGET", KEYS[2], ids[i])

    -- 排序中有 ID，但卡片不存在，不能返回残缺的一页。
    if not card then
        return {"rebuild"}
    end

    cards[#cards + 1] = card
end

-- 7. 返回状态、列表元数据、翻页位置和卡片。
return {"ok", epoch, rev, unread, nextID, cards}