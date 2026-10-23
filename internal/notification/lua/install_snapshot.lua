
-- 1. 检查重建资格是否仍属于自己。
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
    return 0
end

-- 2. 解析旧状态，检查当前 state 是否发生变化。
local old = cjson.decode(ARGV[3])
local kind = redis.call("TYPE", KEYS[2]).ok

if ARGV[2] == "0" then
    -- 开始时不存在，现在也必须不存在。
    if kind ~= "none" then
        return 0
    end
elseif ARGV[2] == "1" then
    -- 开始时存在，现在必须仍然是 Hash。
    if kind ~= "hash" then
        return 0
    end

    local n = 0

    for key, val in pairs(old) do
        if redis.call("HGET", KEYS[2], key) ~= val then
            return 0
        end
        n = n + 1
    end

    -- 除了旧字段的值相同，也不能多出新字段。
    if redis.call("HLEN", KEYS[2]) ~= n then
        return 0
    end
else
    return redis.error_reply("invalid state existence flag")
end

-- 3. 正常格式的旧版本不能高于准备安装的快照版本。
-- 不转浮点数，通过字符串长度和字典序比较整数。
local rev = ARGV[5]
local prev = old["revision"]

if prev and
   (prev == "0" or string.match(prev, "^[1-9][0-9]*$")) then
    if #prev > #rev or (#prev == #rev and prev > rev) then
        return redis.error_reply("snapshot revision is behind cache")
    end
end

-- 4. 在删除旧数据之前，检查卡片参数数量并编码 reset。
local count = tonumber(ARGV[7])

if not count or count < 0 or count ~= math.floor(count) or
   #ARGV ~= 7 + count * 2 then
    return redis.error_reply("invalid snapshot card count")
end

local data = cjson.encode({
    epoch = ARGV[4],
    revision = rev
})

-- 5. 删除旧缓存和旧历史。
-- 不删除 KEYS[1]，重建资格仍由 endRebuild 释放。
redis.call("DEL", KEYS[2], KEYS[3], KEYS[4], KEYS[5])

-- 写入未完成标记；后续任一步失败，都不能把缓存当作有效。
redis.call("HSET", KEYS[2], "applying_revision", rev)

-- 6. 安装完整通知列表。
local pos = 8

for i = 1, count do
    local id = ARGV[pos]
    local card = ARGV[pos + 1]

    redis.call("HSET", KEYS[3], id, card)
    redis.call("ZADD", KEYS[4], id, id)

    pos = pos + 2
end

-- 7. 在新的历史中记录恢复基线。
local id = redis.call("XADD", KEYS[5], "*",
    "revision", rev,
    "name", "notification.reset",
    "data", data
)

-- 8. 保存新缓存状态。
redis.call("HSET", KEYS[2],
    "epoch", ARGV[4],
    "revision", rev,
    "unread_count", ARGV[6],
    "base_revision", rev,
    "base_id", id
)

-- 9. 通知在线连接：缓存已重建，需要重新同步。
local msg = cjson.encode({
    id = id,
    name = "notification.reset",
    data = data
})

redis.call("PUBLISH", KEYS[6], msg)

-- 10. 全部完成，清除未完成标记。
redis.call("HDEL", KEYS[2], "applying_revision")

return 1
