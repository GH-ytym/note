# 本地观察 JWT 和 Redis

后端启动时自动执行 SQLite 迁移。Redis 地址默认 `127.0.0.1:6379`。
access JWT 当前有效期为 60 分钟，refresh token 有效期为 7 天。

## 先用脚本走一遍

在项目根目录打开 PowerShell 7：

```powershell
./scripts/debug-auth.ps1 -PauseEachStep
```

不传 Account 时，脚本注册一个新的 `debug_...@example.com` 本地测试用户，
密码为 `Debug_12345`。测试账号保存在 SQLite；脚本完成时撤销本次刷新会话。
每次按 Enter 后发送下一步请求，输出 JWT 的载荷字段、Redis 键、用户 ID 和 TTL，
不打印签名密钥或完整令牌。

测试自己的账号时：

```powershell
./scripts/debug-auth.ps1 -Account '你的账号或邮箱' -Password '你的密码' -PauseEachStep
```

可选 `-CreateTodo` 会额外创建一个调试群组及一条 Todo，数据保留供检查。

## 使用断点观察

直接使用正式后端、`data/note.db` 和现有 Redis，不再启动独立的 8081 服务。
工作区已有 `.vscode/launch.json`，选择「附加到正在运行的正式后端」并按 F5，
然后选择正在监听 8080 的 `api.exe` 进程。
需要编辑器的 Go 扩展和 Delve；本机已安装 Delve。
`data/debug.env` 保存本地随机密钥和启动配置，data 目录被 Git 忽略。

如果附加后某些变量显示已优化，先停止原后端，再选择「启动正式后端并调试 (8080)」。
这种方式由 Delve 重新编译并启动，便于查看局部变量；数据库和 Redis 仍用正式服务配置。
启动方式不要与已运行的 8080 实例同时使用，否则端口会被占用。

另一个 PowerShell 窗口执行：

```powershell
./scripts/debug-auth.ps1 -PauseEachStep
```

建议断点按顺序设置：

1. `internal/handler/auth_handler.go`：Login 中 `h.tokens.Generate(user.ID)`。
   看 `user.ID`，按 F11 进入 Generate，检查 `claims.UserID`、IssuedAt、ExpiresAt。
   返回后，`tokenStr` 才是最终 access JWT。
2. Login 中 `h.refresh.Replace(ctx, oldRaw, user.ID)`。
   在首次登录时 oldRaw 为空。进入 Replace，观察随机 raw、refreshKey(raw)、userID。
   执行 Redis 脚本后，对应的刷新会话才存在。
3. Login 中 `h.setRefreshCookie(c, refreshToken)`。
   执行后，刷新令牌写入响应 Set-Cookie。JSON 返回的是 access JWT。
4. `internal/middleware/auth.go`：`claims, err := tm.Parse(parts[1])`。
   GET /api/groups 会走这里。看 claims.UserID，再单步到 c.Set。
   Parse 校验 JWT，不查询 Redis；普通业务请求不轮换刷新会话。
5. AuthHandler.Refresh 中 Lookup 和 Rotate 调用。
   Lookup 找到用户 ID；Rotate 原子写新会话、删除旧会话，之后更新 Cookie。
6. AuthHandler.Logout 中 Revoke 调用。
   执行后当前刷新会话删除，随后响应要求客户端清 Cookie。

F10 单步执行，F11 进入函数，Shift+F11 返回调用者，F5 继续运行。
断点停止期间 HTTP 请求可能显示为等待，这是正常现象。

## 手动检查 Redis

脚本会输出属于本次调试的键。把键复制到以下命令中：

```powershell
docker exec note-redis-learning redis-cli GET 'note:refresh:脚本输出的哈希'
docker exec note-redis-learning redis-cli TTL 'note:refresh:脚本输出的哈希'
```

GET 的值是用户 ID，TTL 的单位是秒；`-2` 代表键不存在。
Redis 键的后缀是刷新令牌的 SHA-256 哈希，Redis 不保存原始刷新令牌或 access JWT。

| 阶段 | access JWT | refresh Cookie | Redis |
| --- | --- | --- | --- |
| 登录成功 | 返回一个 JWT | 设置随机刷新令牌 | 新哈希键 → 用户 ID，TTL 约 604800 秒 |
| 普通 API | 放 Authorization 中验证 | 可以携带，但此接口不使用 | 刷新键不变，TTL 随时间减少 |
| 刷新成功 | 重新签发 | 更新为新刷新令牌 | 删除旧键，写入新键 |
| 退出成功 | 客户端清掉内存里的 JWT | 清除 | 当前刷新键删除 |

退出后，拿旧 Cookie 刷新返回 401；但手动携带已签发且未过期的 access JWT
仍可访问受保护 API，这是当前无 access JWT 撤销机制的实际行为。
JWT 的 iat/exp 精度为秒，同一用户在同一秒签发的 JWT 可能相同；脚本等待一秒再刷新。
解码载荷只帮助观察，验证签名仍由服务端 TokenManager.Parse 完成。

## 手动启动后端

在项目根目录执行：

```powershell
$env:NOTE_JWT_SECRET = Get-Content ./data/debug-jwt-secret.txt -Raw
$env:NOTE_REDIS_ADDR = '127.0.0.1:6379'
$env:NOTE_COOKIE_SECURE = 'false'
$env:HTTP_ADDR = '127.0.0.1:8080'
go run ./cmd/api
```

端口被占用时先停掉原实例或换端口。日志出现 `HTTP server listening` 代表开始监听，
访问 `http://127.0.0.1:8080/api/ping` 可以确认是否就绪。

## 这次迁移的范围

空 Todo 表同步 GroupID、CreatorID、外键及 todo_members 表；移除旧标题/正文唯一索引。
有归属的 Todo 补齐缺失的权限记录，保留已有 editor/viewer。
有旧 Todo 但缺少群组或创建者时停止迁移，要求先明确归属；不会自行分配给某个用户。
迁移前的本地 SQLite 备份位于 `data/note.before-group-migration.db`。

创建链路已检查群成员资格；列表、详情、搜索、日历和修改链路的权限仍需后续改造。
