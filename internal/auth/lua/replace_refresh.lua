
local created = redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2], "NX")
if not created then return 0 end
if ARGV[3] == "1" then redis.call("DEL", KEYS[2]) end
return 1
