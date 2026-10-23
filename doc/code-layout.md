# 代码布局与 Lua 脚本索引

整理日期：2026-10-23。本次调整源码组织与脚本加载方式，保留原有业务行为和注释。

## 项目边界

| 目录 | 职责 |
| --- | --- |
| `cmd/api` | 后端程序入口 |
| `internal/app.go` | 依赖组装、启动与退出 |
| `internal/*migration*.go` | 数据库迁移；暂留原包以保留现有内部函数和测试关系 |
| `internal/auth`、`group`、`todo`、`event`、`profile`、`search`、`calendar` | 按业务领域组织服务与数据访问 |
| `internal/notification` | 通知事务、缓存、投递和在线转发 |
| `internal/model` | 共享数据库模型 |
| `internal/handler`、`router`、`middleware` | HTTP 接口、路由和中间件 |
| `web` | 前端 |
| `desktop` | 桌面端运行与窗口管理 |

目前不增加通知子包：缓存、协议和恢复仍共享 Card、Inbox、Change、Event 等类型，重建协调尚未完成。先按职责拆文件，避免为了跨包访问而暴露原本私有的 lease 和 key 构造函数。认证与通知的 Lua 分别归属各自业务包，不建立公共 Redis 工具包。

搜索模块的 HTTP handler/DTO 目前放在 `internal/search`，与其他模块的布局略有不同，但不影响本次 Redis 整理；本次没有迁移其包路径。前端和桌面端也没有变更。

## 通知模块

```text
internal/notification/
  dto.go                   Card、Inbox、Change
  change.go                在业务事务中生成增量 Outbox
  repository.go            SQLite 读取、快照与 Outbox 状态
  service.go               查询服务

  cache.go                 RedisCache、构造函数、缓存 key
  cache_init.go            InitSnapshot：首次初始化
  cache_apply.go           ApplyResult、ApplyChange：增量应用
  cache_rebuild.go         lease、beginRebuild、endRebuild
  cache_install.go         installSnapshot：条件安装快照
  scripts.go               Lua 源码的 go:embed 声明
  lua/
    init_snapshot.lua
    apply_change.lua
    begin_rebuild.lua
    end_rebuild.lua
    install_snapshot.lua
    publish_notice.lua     旧发布器使用的脚本

  delivery.go              投递接口与骨架
  outbox_worker.go         后台投递循环，目前仍调用旧发布器
  redis_publisher.go       旧发布器
  redis_history.go         旧频道体系的历史读取
  redis_subscriber.go      v2 频道订阅与转发
  hub.go                   用户连接管理与分发
  snapshot.go              旧 EnqueueInboxSnapshot 实现
```

初始化和增量应用从原 `cache.go` 移到独立文件，仍属于 `notification` 包，无需修改调用方的 import。既有的旧实现仍保留，避免把目录整理与协议迁移混成一次变更。

## Lua 如何加载

每个包的 `scripts.go` 使用 `//go:embed lua/文件名.lua` 将脚本读入字符串；对应 Go 文件继续用 `redis.NewScript(字符串)` 创建脚本对象。Go 负责参数校验、KEYS/ARGV 组装和返回值处理，Lua 负责 Redis 内的检查与更新。

Lua 在编译时进入可执行文件，不使用运行时 `os.ReadFile`，不依赖启动时工作目录。构建时需要这些源码文件，修改 Lua 后需要重新构建。已有 `Script.Run` 的执行与缓存机制保持不变。

认证包采用相同布局：

```text
internal/auth/
  refresh.go
  scripts.go
  lua/
    replace_refresh.lua
    rotate_refresh.lua
```

搬出时逐一比较了全部 8 段 Lua：统一换行后内容与整理前一致，脚本内注释未修改；原有 Go 注释也保留。

## 当前功能边界

- 本次文件整理不代表恢复流程或应用推送链路已经接通。
- `Rebuild` 协调入口、reset 订阅处理、Worker 切换仍是后续工作。
- `InitSnapshot` 仍保留，待恢复入口统一后再决定清理。
- 不因移动 Lua 文件而改变 Redis key、频道、参数顺序或协议。

## 验证方法与已有问题

直接受影响的认证和通知模块运行 `go test -p 1 ./internal/notification ./internal/auth -count=1`。

后端回归使用 `go test -p 1 ./internal/... ./cmd/api`，并与改动前结果比较。本机并行链接曾出现 Windows 内存分配失败，因此验证时降低并发。

整理前已有 `TestAppPushesJoinApplicationAndShutsDown`、`TestEventGroupJoinQuitAndDismiss` 失败，分别涉及尚未接通的推送链路和暂缓的群解散流程。`go test ./...` 另有 `tmp` 目录两个程序重复声明 main 的编译问题，本次没有删除或移动用户的临时程序。

整理后验证结果：认证与通知模块测试通过；后端回归仍仅有上述两个测试失败，与整理前一致；`go build -p 1 ./cmd/api` 构建验证通过（验证产物写入系统临时目录）；`git diff --check` 通过。本次未提交或推送。
