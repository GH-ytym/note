
local uid = redis.call("GET", KEYS[1])

if not uid or uid ~= ARGV[1] then
	return 0
end

local created = redis.call(
	"SET",
	KEYS[2],
	uid,
	"EX",
	ARGV[2],
	"NX"
)

if not created then
	return -1
end

redis.call("DEL", KEYS[1])
return 1
