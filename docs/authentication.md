# 登录、自动恢复与退出

后端需要 Redis；开发默认地址为 `127.0.0.1:6379`，可用
`NOTE_REDIS_ADDR`、`NOTE_REDIS_PASSWORD` 配置。普通 API 启动需要设置至少
32 字节的 `NOTE_JWT_SECRET`。Electron 若未配置该变量，会在自己的数据目录生成
并沿用密钥。线上 HTTPS 设置 `NOTE_COOKIE_SECURE=true`；本地 HTTP 使用 false。

## 接口

- `POST /api/auth/register`：`{name, email, password}`。
- `POST /api/auth/login`：`{account, password}`，返回用户资料和 `access_token`。
- `POST /api/auth/refresh`：无需请求体，使用 `note_refresh` Cookie 换取 JWT。
- `POST /api/auth/logout`：无需请求体，撤销当前刷新会话并清 Cookie，成功返回 204。

登录、刷新和退出均要求 `X-Note-Request: 1`，并使用同源请求。
不要向任意外部 Origin 开放带凭据的 CORS。
接口也保留无 `/api` 前缀的别名。

## 前端流程

`AuthGate` 在加载业务组件前恢复登录：401 显示登录表单，网络/服务故障显示重试。
登录后 JWT 仅保存在内存，业务请求通过 `api.ts` 携带 Bearer token。
401 时合并刷新操作，用新 JWT 重试原请求一次，避免无限循环。
随机刷新令牌保存在 HttpOnly、SameSite=Strict 的持久 Cookie，Redis 仅存哈希键。
当前 JWT 有效期 60 分钟，刷新会话每次轮换后有效期重新计算为 7 天。

在“设置 → 账号”退出。服务端撤销成功后清空 JWT、用户资料并卸载业务组件。
退出失败会保留状态并显示重试提示。浏览器通过 Web Locks 协调认证请求，
BroadcastChannel 同步同源标签页状态；使用支持这些 API 的现代浏览器。

Electron 的认证请求统一经主进程 AuthSession 排队，并通过默认持久 Session
发送 Cookie 和将 Cookie 刷入磁盘。JWT 不写入磁盘。所有窗口同步登录状态，
退出时关闭辅助窗口并停止提醒；提醒请求也携带 JWT。

## 验证

```text
go test ./internal -run "Test(Auth|Register|Refresh)" -count=1
go test ./internal/auth ./internal/handler ./internal/middleware ./internal/router
cd web && npm run build && npm test
cd desktop && npm test
```

认证 Go 测试使用 miniredis，不连接开发数据库或生产 Redis。
本地浏览器验收：注册并登录 → 重载仍显示日历 → 设置/账号退出 → 重载仍需登录。

## 已知边界

退出撤销刷新资格，已签发 JWT 在服务器上仍有效至过期，不是立即踢下线。
轮换成功但网络响应丢失时可能需要重新登录；未实现刷新令牌家族重放检测。
Redis Lua 面向单实例 Redis。Redis 需作为独立服务部署，桌面包不内置 Redis。
桌面完整重启、Cookie 落盘恢复还应在打包版本中验收。
