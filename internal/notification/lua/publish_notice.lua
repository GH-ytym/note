
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
