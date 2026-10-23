
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
