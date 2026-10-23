
-- KEYS[1]：重建资格 key
-- ARGV[1]：本次重建的 token

-- 先检查资格是否仍属于本次重建
local token = redis.call("GET", KEYS[1])

if token ~= ARGV[1] then
    return 0
end

-- 属于自己才允许删除
return redis.call("DEL", KEYS[1])
