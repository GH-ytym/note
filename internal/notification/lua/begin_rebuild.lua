
-- KEYS[1]：重建资格 key
-- KEYS[2]：缓存 state key
--
-- ARGV[1]：本次 token
-- ARGV[2]：资格有效期，毫秒

--如果被卡住key了直接返回
if redis.call("EXISTS", KEYS[1])==1 then
return{"busy"}
end

--读取state类型
local kind=redis.call("TYPE",KEYS[2]).ok
--要么是不存在要么是hash
if kind~="none" and kind~="hash" then
return redis.error_reply("invalid cache state")
end

--保存当前state
local e="0"
local old={}

--如果有kind那么exist应该是1
if kind == "hash" then
e="1"
old=redis.call("HGETALL", KEYS[2])
end

-- 设置本次重建资格和ttl
--ttl挂在rebuild上面
local ok = redis.call(
    "SET", KEYS[1], ARGV[1],
    "NX", "PX", ARGV[2]
)

if not ok then
    return {"busy"}
end

-- 返回取得资格时的缓存状态。
return {"ok", e, old}
