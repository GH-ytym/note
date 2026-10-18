# 通知后端改造：源码复核与助手执行计划

> 复核日期：2026-10-09。适用仓库：`F:/PROJECTS/go1/note`。
>
> 读者：后续负责指导改造的助手。本文是留在项目内的技术执行依据，独立于用户已带走的入门版计划。
>
> 本次只复核源码、运行已有测试和编写文档，没有实现本计划中的业务改造。

## 0. 每次继续前先读：用户约束

- 用户是初学者，**只写后端，不负责前端**。
- 后续采用“助手解释并贴代码，用户自己写”的方式。没有新的明确授权，不替用户直接修改 Go、前端或配置代码。
- 文档维护、源码阅读和必要验证可以进行。贴过的代码不代表用户已经写入，必须先核对实际文件。
- 每次讲一个小目标，最多引入少量概念，给出文件位置、代码和验证方法，不一口气讲完整分布式系统。
- 用户希望固定思路，允许大改和加入 MQ，但不要因一次报错改换框架、协议或数据库。
- 用户想学习 RabbitMQ，并希望云端运行、减少 SQLite 查询。正常通知读取和断线恢复由 Redis 承担。
- 前端只交付协议、样例和联调要求；React、页面交互、Electron 远程连接改造不属于用户的学习任务。
- 不以“双开网页看起来正常”作为后端唯一验收。应能用 Go 测试、HTTP 请求、SSE 测试客户端独立完成验收。

用户原始目标：申请入群、指定用户邀请、接受/拒绝审核、相关人员实时收到结果；离线后上线可恢复，多窗口同步。不是开发聊天消息系统、全站事件平台或无限期事件溯源。

## 1. 审查结论及相对入门版的修订

**结论：主路线可行，保留 SQLite → Outbox → RabbitMQ → Redis → Hub/SSE。入门版不是可直接照抄的完整实现规范，必须加上下面的后端约束。**

| 入门版的不足 | 本版决定 |
| --- | --- |
| 把 React 改造列成用户步骤 | 移出后端教学；使用 Go 客户端验证协议，单列前端交接清单 |
| 审核、邀请放在很多基础设施步骤之后 | 前移数据库业务闭环，先用真实申请/邀请变化验证协议，再接缓存和 MQ |
| 假定接受申请只需写成员表 | 必须复用现有 addMember，初始化 Todo/Event 权限，保持原行为 |
| 未具体处理群主转让、已有成员与多个待处理请求 | 加入业务规则清单，不能遗漏这些已有路径 |
| 资料变化泛化成改名功能 | 当前只发现头像修改接口，先接 SetAvatar；不为通知改造新增昵称或群名修改功能 |
| “过期”只笼统提及 | 截止时间必须由后端校验；展示状态变化要入事务和版本，不能同版本随时间悄悄变内容 |
| Redis 降级与当前启动逻辑冲突 | 明确拆分 HTTP 可用性、认证可用性和通知同步健康；修改相应启动测试 |
| 只描述服务端检查客户端位置 | SSE 没有应用层 ACK；只能知道最后成功写入的位置，不能声称知道 UI 已应用 |
| 最近 50 条可能被误解为所有离线通知 | 它只是同步窗口；持久历史仍在 SQL，超过窗口的旧通知需分页访问约定 |
| MQ 死信似乎就是可靠归档 | 死信路由有自身可靠性边界，先持久记录故障/修复任务，不静默丢弃 |
| SQL/MQ/Redis 每层均已确认似乎等于端到端完成 | 明确业务提交、broker 接管、缓存应用、SSE 写入、用户已读是不同阶段 |

这些修订收紧实现和教学范围，不更换架构。入门版文件保留原样，避免用户带走的副本被悄悄改成另一套内容。

## 2. 源码核对结果：事实而不是设计设想

行号仅供本次定位；后续以函数名搜索，避免代码变动后误用行号。

### 2.1 已有能力与故障

| 源码位置 | 2026-10-09 实际状态 |
| --- | --- |
| `internal/group/repository.go:149` / Join | public 直接加入；approval 新建申请；7 天内同一 pending 申请复用；两条写入路径已调用 EnqueueInboxSnapshot |
| `internal/group/repository.go:316` / enqueueNotice | 旧 Card 入队函数仍在，但已不是上述两条路径使用的函数 |
| `internal/notification/snapshot.go:39` | NotificationState 按不存在的 receiver_id 查询，应该是 user_id |
| `internal/migration.go` / migrateDatabase | 调用了申请、通知、Outbox 迁移，没有 NotificationState 迁移 |
| `internal/group_migration.go` | 通知/Outbox 表存在即返回，修改模型标签不会升级已有表 |
| `internal/model/outbox.go` | 新增 receiver_id + revision 唯一约束；旧写入未分配版本，默认 0 会冲突 |
| `internal/notification/service.go:52` | List 返回 Items、UnreadCount，没有装入真实 Revision，输出仍为零值 |
| `internal/notification/repository.go` | 列表和全部未读数处于同一 SQL 读取事务；关联加载 Actor/Group/JoinRequest；最近 50 条按 id DESC |
| `internal/notification/dto.go` / NewCard | 展示当前关联资料；RequestStatus 来自申请，不是通知自有状态 |
| `internal/model/notification.go:12,37` | 有 JoinReviewed 常量，但数据库 CHECK 只允许 joined、join_requested |
| `internal/notification/redis_publisher.go` | Lua XADD + sent Hash 去重 + PUBLISH；没有当前面板缓存、版本连续检查、保留上限 |
| `internal/notification/redis_history.go` | 一次 XRANGE 最多 100 条，按 Redis ID 查询；没有完整恢复判定或 SSE 调用 |
| `internal/notification/redis_subscriber.go` | PSubscribe 所有用户 live 频道，解析、校验接收者、转发 Hub；未检测静默漏消息 |
| `internal/notification/hub.go` | 每连接 32 个事件缓冲；满则结束订阅；可复用 |
| `internal/handler/notification_handler.go` | JWT 到期结束连接、15 秒心跳、写超时和 Flush 可复用；无断点读取和恢复状态机 |
| `internal/router/router.go` | 有通知列表与 stream、加群、退群、解散；无邀请发送、审核、通知已读接口 |
| `web/src/lib/notification-stream.ts` | 只接受 notification.created/updated，不消费 Card、revision、id；忽略 notification.snapshot |
| `web/src/hooks/useNotifications.ts` | 已有串行刷新/dirty 合并、重连后刷新、聚焦和 60 秒 SQL 校正 |

不要再称“现有完整同步链路已经可用”。之前双窗口成功属于较早的 Card+回查版本；当前工作区已被半接入快照改动影响。

### 2.2 必须保留的业务语义

- `group.addMember` 同时创建 GroupMember、TodoMember、EventMember。已有内容的新成员默认 viewer；用户退群后再加入，对自己仍是 creator 的内容恢复 editor。
- `group.Quit` 包含群主转让，随后清理退出者的内容权限；不可因通知改造破坏此流程。
- `group.Dismiss` 清理群内内容再删群；通知、申请依赖外键级联。必须删除前收集通知接收者，删除后维护他们的状态与 rebuild 任务。
- `model.GroupPolicy` 注释已定义：public/approval 群成员可邀请，接受邀请即加入；restricted 仅群主可邀请；personal 不可邀请或主动申请。不要把 approval 邀请擅自变成两级审核。
- `GroupJoinRequest.ReceiverID` 在 application 是当时群主，在 invitation 是被邀请人。不能把相同字段误当成所有场景的入群用户。
- 新增请求目前只因 7 天条件不再复用旧 pending，旧记录没有自动变为 expired。
- 申请表的 Sender/Receiver 引用用户使用 RESTRICT，群删除 CASCADE；通知申请引用 SET NULL，通知群引用 CASCADE。保持迁移数据完整性。

### 2.3 现有运行约束

- `app.go` 将 SQLite 连接池设置为 1，使用 WAL、busy_timeout、immediate 事务，当前 synchronous=NORMAL。
- **事务内一律用传入的 tx。**在持有唯一连接时调用根 db 或另一个自动开启事务的 repository，可能等待自己释放连接。
- Redis 同时服务认证刷新会话和通知。共享客户端设置 `MaxRetries:-1` 有认证轮换安全原因，不能为通知重试随意修改。
- 当前启动先 Ping Redis、确认订阅，失败会退出；运行中 subscriberErr 也使整个 API 停止。
- `TestAppDoesNotReportReadyWhenSubscriptionFails` 明确锁定了现有启动行为。降级改造时要解释行为变化并更新测试，不能偷偷删测试。
- `desktop/main.cts` 会启动本地 Go 子进程并连接本地端口。共享云端 API 不会因为后端加了 MQ 就自动被桌面端使用；这是外部客户端部署对接项。

### 2.4 本次实际验证

执行：`go test ./... -json`，输出临时保存在 `tmp/notification-plan-audit-tests.jsonl`（本地诊断记录，可被忽略或清理，本文保存结论）。

- `note/internal` 有 **11 个顶层测试失败**，子测试不重复计数。主要是 notification_states 缺表与 Outbox revision=0 冲突。
- 失败测试包含应用级 SSE 测试、入群/权限初始化测试、通知列表、Outbox 创建与重投测试。
- 其余有测试的 13 个子包本次通过；通知子包通过不表示应用级链路通过。
- `note/tmp` 额外编译失败：已有 `tmp/demo-password.go` 与 `tmp/reset-notification-redis.go` 同属一个包，重复定义 main。本次没有创建或执行这两个程序。
- 没有连接用户开发 Redis、清理 Redis、安装 RabbitMQ 或修改现有数据库。

因此阶段 0 应分别处理业务回归和临时程序目录污染。未来可把教学程序放在不同 `cmd/...` 目录或独立示例模块；不要直接删除用户的 tmp 文件。诊断可先使用 `go test ./internal/... ./cmd/api`，但必须说明这不是 `go test ./...` 已通过。

## 3. 调研结论：保留哪些依据，不能扩大哪些承诺

| 一手资料 | 确认的设计 | 本项目借鉴范围 |
| --- | --- | --- |
| [Zulip events system](https://zulip.readthedocs.io/en/latest/subsystems/events-system.html) | 初始数据与先建立的事件队列衔接，过期后重新初始化，测试旧状态+事件=新状态 | 初始同步算法与测试方法；不是 Redis 实现模板 |
| [Telegram updates](https://core.telegram.org/api/updates) | 序列检查、去重、缺口主动补差量、太旧则重新获取状态 | 客户端消费规则；不能推断其内部用 Redis |
| [Centrifugo history/recovery](https://centrifugal.dev/docs/server/history_and_recovery) | 有界历史、epoch/offset、明确恢复失败、检测在线漏消息 | Redis 近期历史与恢复边界；不等于替业务维护审核/未读数据 |
| [ntfy API](https://docs.ntfy.sh/subscribe/api/) | SSE/HTTP 流，since 补缓存消息，poll 查询 | Go SSE 与补收接口形式；不是可直接套用的可变 Inbox |
| [Slack real-time messaging](https://slack.engineering/real-time-messaging/) | 业务处理、频道分发、连接网关分工 | 不让每连接重查业务数据库；不照搬多地域微服务 |
| [RabbitMQ confirms](https://www.rabbitmq.com/docs/confirms) | 发布确认与消费 ACK 独立；需处理重投及不可路由消息 | 后台可靠任务交接，不承诺恰好一次或浏览器已收到 |
| [RabbitMQ DLX](https://www.rabbitmq.com/docs/dlx) | 死信路由有可靠性限制，目标不可用时不能笼统保证保存 | 故障持久记录、确认后的停车转发，不依赖一句“进死信就好” |

前期几次方案切换的纠正：Redis-first 指读取与恢复优先，不是把申请审核先写缓存再异步落库；完整面板用于初始化/重置，普通变化使用增量；MQ 不能替代 Outbox 或浏览器同步协议。

RabbitMQ 是学习与后台调度的选择，不是减少 SQL 的必要条件。既已选定，不在教学中反复讨论 Kafka/NATS/Redis Streams consumer groups 等替代实现。

## 4. 固定架构与范围

```text
业务 HTTP -> SQLite 事务（业务 + 通知 + 用户状态 + Outbox）
          -> 事务后唤醒发布器，另有扫描兜底
Outbox 发布器 -> RabbitMQ（确认后记 broker_confirmed_at）
RabbitMQ 消费者 -> Redis 面板 CAS + 事件历史 + Pub/Sub
               -> 确认完成后 ACK
订阅器 -> Hub -> 带初始化/恢复逻辑的 SSE

查询：Redis 面板优先，缺失/无效/必要一致性要求时 SQL 重建
恢复：Redis 近期变化优先，不完整则读取面板快照
校验：在线用户 Redis head 检查 + 低频 SQL revision 对账
```

单 Go 进程、单 SQLite 所有者、单实例 Redis、一个 RabbitMQ 通知工作队列。先不实现分片、多活、Redis Cluster、分布式事务或每窗口 MQ 队列。

后端交付边界：业务 API、缓存、后台任务、SSE 协议、测试、部署说明。前端修改由其他负责者完成；后端完成不能表述为用户界面已经上线。

## 5. 后端业务设计清单

### 5.1 先做最小闭环，不新建通用工作流平台

1. 申请：沿用群号、最新 policy、已有成员判断、重复 pending 处理。
2. 审核：当前有权限者接受/拒绝，更新请求状态，接受时复用 addMember，生成申请人的结果通知，并更新所有引用此申请的通知接收者。
3. 邀请：按当前 policy 验证邀请者；被邀请者接受/拒绝；接受时从 invitation.ReceiverID 确定新成员。
4. 已读：仅接收者，重复标记幂等。已读不改变申请状态，审核也不偷偷等同已读。

邀请通知需要新增明确 Type，如 group_invited / invitation_reviewed，具体字符串在协议阶段一次固定，并更新 CHECK、迁移、样例和测试。避免复用 join_requested 让前端猜 Kind。

### 5.2 事务内处理顺序

- 校验身份、当前角色、群策略、请求有效期。
- 条件更新 pending -> 目标状态并检查 RowsAffected。
- 写成员和 Todo/Event 权限（需要时）。
- 新建/更新持久化通知。
- 收集全部受影响用户，排序去重，每用户本事务只推进一次 revision。
- 在 tx 内统计未读总数并持久化到 state，生成该版本任务。
- 成功提交后才唤醒 Outbox 发布器；事务内不能访问 MQ、Redis 或 COS。

相同决定的重试不生成新通知、新版本；与已完成决定相反的请求返回明确冲突。HTTP 超时不能被当成事务一定没提交，重试必须安全。

### 5.3 必须补测的业务分支

- 成员已经通过其他方式入群，再接受旧邀请/申请；不能再次 addMember。
- 同一目标用户有多份邀请或申请；一份成功后，其他 pending 如何关闭、通知哪些人。
- 群主转让后，旧群主失去审核权，新群主能获取待审核任务。不能只改 owner_id，仍把所有待办发给旧接收者。
- 邀请者退群或不再具备邀请权限，旧邀请是否仍可接受。
- personal、restricted、public、approval 的权限矩阵。
- 解散前收集所有相关通知接收者，包括尚未入群的申请者/受邀者。

其中“旧请求转交还是取消重建”“邀请者失权后邀请是否有效”是源码尚未定义的产品边界。到实现该分支前，用具体例子向用户一次询问；不要自行把架构讨论扩成权限设计，也不要因此停下其他独立阶段。

### 5.4 过期不是仅靠 Redis TTL

现有 7 天规则必须保留。后端每次接受/拒绝前校验截止时间，过期不能批准。

推荐显式持久化 expired 状态，并有可注入时钟的到期扫描；需要增加数据库 CHECK 允许值。最晚在业务阶段确认这项可见状态名称；不能拿 Redis key 过期等同申请过期。

到期状态推进、关联通知更新、用户 revision 与 Outbox 必须一起提交。不要在 NewCard 内按 time.Now 改写状态而不推进版本。到期扫描尚未执行也不能让审核接口放行；校验失败时如需保存过期变化，不能返回错误导致整笔状态事务又回滚，应区分提交结果与 HTTP 业务错误。

### 5.5 当前资料联动与作用域控制

实际必须修改 `profile.repository.SetAvatar`：头像确有上传和删除 API。在同一数据库事务中找关联通知接收者并生成 rebuild。

只在头像 URL 实际改变时推进版本。可评估为 notifications.actor_id 增加索引，避免全表找关联用户。保持 COS 上传/删除位于事务外；SQL 失败仍按原 service 清理新对象。

当前未发现昵称、群名修改 API。为未来入口留规范，不凭通知需求新增这些功能。对应直接修改数据库的测试应通过业务入口或明确模拟完整事务，否则会人为破坏版本约定。

## 6. SQL、任务与迁移约束

### 6.1 状态与任务

`notification_states(user_id, revision, unread_count)`：每用户一行；新用户注册事务内创建版本 0、未读 0 的状态。迁移为所有缺状态的老用户补齐，有旧通知则统计真实未读数，否则为 0。EnqueueChange 不补建状态、不执行 COUNT，缺状态时报错回滚；登录不重置计数。

revision 使用非负、SQLite 有符号整数范围内的值；JSON 编码为十进制字符串。不要因为 Go 当前字段为 uint64 就假定 SQLite 能保存全部 uint64。

任务包含 task_id、user_id、revision、schema_version、kind、完整增量 Card 集合、绝对 unread_count。kind 第一版只有 change/rebuild。增量 payload 在原业务事务确定，不能消费者读取最新 Card 后却贴上旧版本。

Outbox 与业务对象没有会导致群解散时任务被级联删掉的依赖。幂等身份使用任务 ID 和用户版本，不能复用 RabbitMQ channel 范围内的 delivery tag。

### 6.2 迁移门槛

- 首先保存当前工作区差异，不使用 git reset 清掉用户未提交改动。
- 用临时 SQLite fixture 覆盖：新库、旧 Card Outbox、有/无半成品 revision 字段、有重复 0 的旧任务、重复启动。
- 表存在即 return 不再适合升级 CHECK 和字段。设计显式迁移并验证 FK、索引、记录数、主键、已读时间和请求关联。
- 新版生产消费前，旧格式任务必须转入明确的兼容处理或原子生成替代 rebuild 后归档。不能给旧任务随意捏造一条完整历史。
- 部署停旧 worker，再迁移和启用新 worker，避免两套协议同时写同一缓存；通知 Redis 使用 v2 命名空间与旧缓存隔离。
- 不删除真实数据库，不自动执行现有 tmp/reset-notification-redis.go，不用 FLUSHALL 处理通知缓存（同 Redis 还有登录会话）。

### 6.3 一致读取与查询开销

LoadSnapshot 必须在一次 DB 读取快照内拿 state、最近 50 条与关联资料。COUNT 仅用于状态初始化、真实业务变更和审计修复，正常缓存 GET 不重复 COUNT。

状态、卡片和关联资料都在同一读事务才可信。不要把仓库方法逐个调用后拼在一起称为同一版本。

未读索引已有 `(receiver_id, read_at)`；最近通知查询、待发布任务扫描、资料关联索引在实际 SQL 上 EXPLAIN 后决定，不能仅凭模型标签宣称有正确索引。

## 7. Redis 执行规范

### 7.1 初版结构

沿用入门版选择：单个最多 50 条的面板 JSON + Stream 历史 + 必要的有效性/恢复元数据。缓存键使用通知 v2 前缀并按 user_id 分组。只做单实例，不因花括号槽位写法就宣称已支持集群。

JSON 面板使读取简单一致，但每次更新会在 Go 和 Redis 之间读写一个有上限的快照。这是明确的性能折中，不是“零成本增量”；客户端仅接收 change，消费者不回查 SQL。

所有安装操作检查 expected epoch/revision；对正常事件仅接受 R+1；<=R 只在缓存有效时忽略，>R+1 重建。客户端和历史恢复另有自己的连续性检查，不把 Redis 成功当成浏览器应用成功。

### 7.2 Lua 原子执行不等于错误自动回滚

预先验证 key 类型、载荷、版本、大小；脚本写入中断或返回不确定错误时，不允许仅因面板 revision 已增加就把该任务 ACK。

需要有效性标记/提交凭据或等价的可验证机制，证明面板和历史确实对应同一已完成提交。检测到局部写入时将缓存标为不可信并从 SQL 重建，不要求补救每种部分写入。

超时可能表示脚本已经成功：重新读取健康状态判断重复，而不是盲目再次 +1 或无条件删除另一条并发更新。

### 7.3 重建与代次

第一版是单进程，先使用按用户合并请求的 singleflight 降低冷启动回源；实际安装仍需条件校验。跨进程重建令牌可以在缓存模块内封装，不要先搭通用分布式锁框架。

获取重建资格 -> 一致读取 SQL -> 原子校验资格和旧缓存条件 -> 安装完整 snapshot、新 epoch 和历史基线 -> 广播 reset。

锁/资格过期、缓存已更新、另一次重建已完成，旧结果不能覆盖。比较整数版本不替代代次验证。

重建读到 SQL 版本 N 后，可以 ACK <=N 的旧任务，因为完整面板已含这些变化；这不能推广为增量任务允许跳号。重建也不能把新 epoch 放在旧历史上继续恢复。

### 7.4 保留、恢复边界与冷缓存

- 明确配置面板闲置 TTL、历史最大条数、历史最大年龄。Redis Stream key 的滑动 EXPIRE 只会删除整个 key，不等于每条事件到龄删除；活跃用户仍需要按时间裁剪历史。
- 初始基线版本、最早可恢复位置与当前 head 必须可判定。历史被删除、裁剪或在分页期间变化时，无法证明完整就 reset。
- 恢复固定一个目标版本，分批取全，再衔接实时缓冲；有最大条数/字节/耗时限制，超过就快照。
- 缓存缺失时消费任务先重建，不能从零应用任意版本；冷用户首个任务回源一次是接受的成本，不能宣称所有发布都零 SQL。
- Redis 数据丢失或回退、SQL/MQ 已经前进时，已 ACK 的任务不会自动回来。低频 DB 对账与按需重建是必要兜底。
- Redis 缓存 revision 高于 SQL revision 视为恢复异常（如数据库回滚恢复），不能继续热读；需要明确新代次重建并记录故障。

## 8. RabbitMQ 执行规范

选择官方维护的 AMQP 0-9-1 Go 客户端 `github.com/rabbitmq/amqp091-go`，不混用 AMQP 1.0 或 RabbitMQ Streams 示例。[官方 Go 入门](https://www.rabbitmq.com/tutorials/tutorial-one-go)

第一版固定一条持久工作队列、一个活跃消费者，prefetch 从 1 开始便于理解。学习阶段可用单节点 durable classic queue，消息 persistent，明确不是多节点容灾。扩容和队列类型变更另行评估。

### 发布器

- 单个活动 relay 读取批次，不为每个窗口运行 relay。
- 事务后唤醒只缩短等待，扫描保证进程崩溃后任务仍能发现。
- publisher confirms + mandatory returns；broker ack 一个不可路由消息不算发布成功。
- 连接/通道重建后重新声明拓扑、注册确认回调，正确关联 task_id 与本次发布序号。
- 先不在一个数据库事务内等待 MQ 确认。MQ 不可用时业务仍可提交，Outbox 积压可见。

### 消费者和失败

- 普通成功、有效缓存下的幂等重复、SQL 重建成功后才 ACK。
- 服务依赖故障退避并限制并发/未确认数；注意 RabbitMQ 的消费 ACK 超时，不能永久捏着一条消息无限等待。
- 不把暂时 Redis 故障当作每条消息的永久毒消息，避免全部打入死信。
- 永久坏载荷：先持久记录失败与受影响用户修复要求，再转入停车/死信队列；如果应用主动转发，等待发布确认后再 ACK 原消息，重复停车也要可识别。
- 默认 DLX 转发不被当作天然无损。修复 SQL 记录是兜底，未记录成功时不静默 ACK 丢弃。
- 重试、单消费者故障接管仍可能重投；用户 revision 连续检查仍是最终防线。
- 停机先停止拉新任务、取消有界操作；未完成任务保持未 ACK，关闭通道让 broker 重投。

队列学习示例不得全部放在同一个 tmp 包里定义多个 main。每个独立例子放独立目录。

## 9. HTTP/SSE 合约：后端负责到哪里

以下为拟定合约，进入协议阶段一次性固定，生成独立的前端交接文档和 Go 测试样例；不是本次已存在的 API。

### 9.1 与现有前端兼容

用户不负责前端，不能新后端发布后让现有页面静默不更新。

建议同一路径明确区分协议：`GET /api/notifications/stream?protocol=2` 提供新同步协议；默认旧协议暂时保持 notification.created/updated 刷新信号语义。兼容层只负责事件映射，不保留第二套数据库和缓存实现。

`GET /api/notifications` 保留 items/unread_count，添加版本等字段是向后兼容的；读取已切到同一个 Redis-first 服务。旧页面仍回查，但热命中不再压 SQL。只有前端负责人完成新协议接入后，才能宣称客户端已取消回查和 60 秒轮询。

保留旧模式不是最终路线反复，而是有明确移除条件的兼容适配；新后端验收使用 protocol=2。

### 9.2 新协议约定

- 身份来自 Authorization Bearer，中间件 user_id，不能让请求指定其他接收者。
- SSE cursor 包含 protocol version、epoch、revision；设置长度、格式和范围限制。优先显式 cursor 参数，Last-Event-ID 兼容时明确冲突规则，不能默默取两个不同断点。
- 无 cursor：订阅本地 Hub -> 读取一致快照 -> snapshot -> 连续处理后续事件。
- 有 cursor：先接实时缓冲，再固定恢复目标并补历史；不足则 reset/snapshot。
- reset 在当前连接内紧接一份 snapshot；客户端据 snapshot 重新建立状态。不能让两端都等对方重新请求而卡住，也避免无限断开重连循环。
- snapshot/change 携带可用于下次重连的 cursor。reset 和 ping 不推进应用游标。
- SQL 降级快照与正常 Redis 快照区分来源，游标为空或标记不可恢复；Redis 恢复后重新发正式 snapshot。不能把 SQL 当前版本伪装成 Redis 已有历史位置。
- JWT 到期、取消和慢连接结束保持原行为；登录刷新仍沿用现有 HTTP 认证流程。

### 9.3 操作后的最小版本要求

审核响应保存业务结果，可附当前操作者的 notification_revision（如果该用户有相应变化）。不返回其他用户游标，不拿它直接推进浏览器消费位置。

提供一致性入口，例如 GET notifications 支持 `min_revision`：Redis 低于要求时，在有界等待后合并 SQL 回源/重建，或明确返回同步暂不可用，绝不能悄悄返回更低版本并表示已满足要求。限制伪造超大版本引起的反复 SQL 查询。

当操作者没有通知变化时，不捏造一个版本；请求返回的业务状态本身已经可用于成功反馈。无需实现前端乐观覆盖/回滚机制。

### 9.4 SSE 写入位置不是客户端 ACK

Handler 仅维护 last_written_cursor，定期按用户获取 Redis head，对比后补发或同步；用内部检查消息唤醒 handler，不把未定义的 checkpoint 当成第四种业务事件发给前端。

写成功只表示服务器把字节交给连接，不证明 UI 应用了。客户端重连携带的是最后成功应用的游标；Go 测试客户端模拟这一规则。第一版不新增浏览器 ACK 存储表。

Pub/Sub 静默断开、重订阅及慢队列丢弃都必须触发恢复判断。检测 Redis 连接恢复后，校正活跃用户；没有下一条事件时也不能永久停在旧版本。

### 9.5 50 条窗口之外

通知永远不要因为 Redis 裁剪而从 SQL 删除。新协议快照只同步最近 50 条及全部未读数。

若需要查看更早的通知，交接文档规定独立的持久列表分页（如 before_id、limit）；该分页允许索引 SQL 查询，不承诺走近期 Stream。前端没有历史入口时，不能称“所有旧申请都可在最近 50 条中处理”。首次上线大量积压的验收需覆盖这一边界。

## 10. 运行生命周期与故障模式

### Redis

当前 Redis 是认证依赖。通知降级只能保证仍有效 JWT 用户可受控查询 SQL；登录/刷新会话依赖 Redis，Redis 全部不可用时这些功能不保证正常。

启动策略要显式调整：数据库/JWT 配置失败仍阻止服务；通知订阅/MQ 暂不可用可启动 HTTP 并报告降级、后台重连。Redis 认证失败不能伪造登录成功。新增健康端点/状态字段区分服务存活与推送健康，不把 NOTE_SERVER_URL 解释为所有后台连接都健康。

使用现有 Redis 客户端的 MaxRetries:-1，不修改认证语义；通知模块执行可判定幂等的显式重试。如确需专用连接池，单独配置并说明，不复制整个 Redis 服务。

### SQLite

单连接意味着业务写入、快照重建、Outbox 扫描互相排队。按需合并、分页/批量、短事务、避免事务内网络调用。

WAL + synchronous=NORMAL 不能直接承诺主机断电后每次已确认事务都绝对不丢。云端上线阶段评估同步级别、备份和性能；对用户准确解释保证范围，不为 MQ 引入而暗改数据库配置。

### 后台循环

现有 subscriberErr 使 API 退出，需改为监督重连与状态上报；重大配置错误与短暂网络故障区分。更新原启动测试的预期，不删除健康检查。

关闭顺序：停止新消费/发布 -> 取消有界等待并释放 SSE -> 等待工作循环结束 -> 关闭 MQ/Redis/DB。不得让旧后台 goroutine 在池关闭后继续使用资源。

### 客户端部署

浏览器 API 地址、Cookie、HTTPS、代理 SSE buffering 是交接项。Electron 当前本地后端启动和 localhost 绑定需客户端负责人修改；用户只做后端，不让其编辑 desktop/main.cts。

## 11. 后端专用实施顺序

每个 B 阶段拆成若干教学小步。下一步只给当下所需代码，后续文件仅说明用途。

| 阶段 | 工作与主要文件 | 独立验收 |
| --- | --- | --- |
| B0 保存现场/恢复基线 | 当前 snapshot 半改动、schema、测试 fixture；保留用户差异，解释临时回查基线 | 申请与 Outbox 测试、应用 SSE 测试恢复；tmp 编译冲突单列处理 |
| B1 MQ 小练习 | 独立 cmd 示例、开发运行说明；本阶段不改业务链路 | 发布确认、ACK 前退出再投递的实际演示 |
| B2 协议/状态/迁移 | model、notification dto/事务辅助函数、migration、repository | 回滚、版本唯一、未读数、新旧库与重复迁移测试 |
| B3 申请/邀请业务闭环 | group repo/service/handler、router、errors、相关通知类型 | Go HTTP 测试完成 A 申请 B 审核/A 邀请 B 接受；验证成员与 Todo/Event 权限 |
| B4 Redis 面板/历史 | cache、脚本、snapshot loader、history/rebuild | 直接模块测试：连续/重复/缺口/冷缓存/50条/旧重建 |
| B5 MQ 真实接入 | relay、consumer、mq connection/topology、失败修复、app wiring | MQ确认丢失、Redis应用后未ACK、断连恢复；真实broker集成测试 |
| B6 SSE 新协议与兼容 | handler、Hub、subscriber、协议测试客户端 | 无浏览器验证 snapshot/change/reset、两用户隔离、多连接、JWT到期 |
| B7 恢复/健康/降级 | history paging、版本对账、监督重连、min_revision、app lifecycle | 断点失效、静默漏尾事件、Redis清空/回退、MQ故障、启动与停机 |
| B8 业务边角/部署/测量 | 资料联动、转让与失效分支、持久历史接口、部署与指标说明 | 关键边界完整；基准SQL次数/命中率/积压；交付前端协议文档 |

B3 的业务可以先直接检验 SQL 与 Outbox，不要求在 B5 之前搭建临时替代 MQ 产品。生产启动未切换前可分模块测试；避免同一环境同时运行新旧两条发布路径。

资料联动、转让、失效等如影响 B3 用例，应提前实现其必要部分，B8 做覆盖补全。不能先上线一个明知可越权审核的“基础版本”。

建议模块仍留在 `internal/notification`，保持 model 不依赖业务包，group/profile 调用只依赖 model 的通知事务辅助代码；不要制造 group ↔ notification 循环导入。初期不抽通用消息总线框架。

## 12. 必须保留的测试矩阵

### 已有测试的价值

- `notification_schema_test.go`：通知外键/级联与数据保留。
- `notification_list_test.go`：50 条与全部未读数、接收者边界、已读和审核分离。
- `notification_outbox_test.go`：业务与任务同事务。其“单 Card created”断言未来按新协议更新，但不能删除回滚测试。
- `notification_worker_test.go`：标记失败再投递、继续发现新任务、取消。
- `notification/hub_test.go`：慢连接不阻塞其他连接、可重新订阅。
- `app_notification_test.go`：真实 HTTP/后台/SSE/退出流程，可扩展为无前端端到端客户端。
- `group_join_test.go`、`event_group_test.go`：入群内容权限，不能因只关注通知而漏掉。

### 新增高价值用例

1. 同一业务重复/并发请求只产生一次状态变更；两种相反审核不能都成功。
2. 接受申请与接受邀请准确识别入群用户，初始化现有 Todo/Event 权限。
3. Redis 20 收到 22 不直接应用；重复 21 不重复历史/计数。
4. 重建 20 与实时 21 竞争，资格过期、缓存删除重建后旧任务返回，均不能覆盖新状态。
5. Lua 面板写入后历史追加失败，不把表面 revision 当成已完整提交。
6. 删除/解散后迟到旧事件不能复活卡片；旧通知30更新不能挤出101～150。
7. 初始快照与审核并发；恢复超过100条；恢复中又有新事件；裁剪发生在分页之间。
8. 丢最后一条 Pub/Sub、Redis订阅自动重连但SSE不断，均可在检查周期内被发现。
9. 已完成MQ消费后Redis清空/回退，SQL可重建；min_revision不能返回更低的热缓存。
10. SSE Flush 不算业务已读，客户端游标20不能因HTTP成功22而跳过21。
11. Redis故障时仍有效JWT的受控SQL降级，与登录/刷新不可用行为分别测试。
12. 老库迁移保留数据且幂等，旧任务替换后不被重复消费；认证Redis键不受通知修复影响。
13. SetAvatar真实变更后缓存与SQL一致；相同URL不凭空产生新版本。
14. 群主转让、请求超期、邀请者失权按确认的规则处理，越权不能靠缓存放行。

单元测试使用临时SQLite/miniredis/可注入时钟。RabbitMQ确认、重投、停机以及Redis Lua/Stream裁剪需要隔离真实服务集成测试；不能只凭 miniredis 通过就宣称与真实Redis完全一致。

所有恢复结果都要与直接查询数据库得到的当前面板比较。还应断言正常热路径没有调用SQL loader，而不只看界面状态。

## 13. 前端交接清单（不交给用户编写）

在 B6/B7 生成独立协议文档，至少包含：

- 请求路径、protocol=2、Bearer鉴权、JWT过期如何重连。
- snapshot/change/reset的完整JSON样例、SSE帧、游标编码与数字字符串。
- 初次打开无本地状态不能只带旧游标；游标在应用成功后推进。
- 同epoch连续应用、重复丢弃、缺口重同步；reset之后snapshot的确定行为。
- 50条排序/截断与全部未读数；更早通知分页。
- 操作成功后的revision下限与pending同步反馈，不能跳版本。
- 新成员加入后哪些群组/成员查询需要刷新；不把这些数据也塞入通知面板。
- 降级响应、错误码、重连退避、旧协议移除条件。

当前前端仍使用回查模式这一事实持续记录，不能要求用户完成 React hooks 作为后端毕业条件。若将来用户授权助手处理前端，应另立明确任务，不能默认本轮已授权实施。

## 14. 防止再次换思路的规则

- 缓存初始化/删除补位回源一次，是本设计内的正常路径，不因此退回“每事件查SQL”。
- 消费版本缺口时SQL重建，是恢复边界，不改成无限Redis历史。
- MQ学习示例不等于生产可靠性完成；没有确认/重投测试就不勾选B5。
- 出现缓存/HTTP状态倒退，先检查epoch/revision/提交时序，不先更换SSE。
- 需要改变本文决定时，写明可复现反例、局部修订和迁移影响。新产品边界只在相关业务步骤确认，不重启整套架构讨论。
- 不把历史展示快照重新偷偷引入；当前按最新资料，资料变更必须有版本联动。
- 不因用户允许大改就越过“贴代码、用户写”的教学方式。

## 15. 后续助手接手记录

### 当前已完成

- [x] 复核现有后端与前端对接状态，回顾一手调研资料。
- [x] 运行当前Go测试并记录失败基线。
- [x] 留存本内部计划，明确后端范围。
- [ ] B0 基线修复。
- [ ] B1 RabbitMQ 练习。
- [ ] B2～B8。

### 当前下一步

目前 B2 已有 Change、状态迁移和经过测试的 EnqueueChange 增量入队辅助函数；下一步处理接入前计数校准、版本约束与事件兼容，尚未切换 Join 或 MQ。

每完成一个小步追加：日期、实际文件、用户写入状态、测试命令与结果、尚未解决的问题、下一步。保留“当前测试失败”与“新代码尚未写入”两种状态的区别。

### 教学记录：2026-10-09，B0 第一个小步已给出

- 再次读取实际源码与差异，确认 Join 仍调用未完成的 EnqueueInboxSnapshot，Outbox 仍有 receiver_id + revision 唯一索引标签。
- 给用户的改动：Join 两处恢复调用已有 enqueueNotice；Outbox 保留 Revision 字段但暂用 default:0，移除唯一索引标签并纠正快照注释。只恢复现有 Card 事件基线，不实现新同步协议。
- 用户写入状态：尚未确认；助手未修改 Go 代码，未验证所贴修改后的测试结果。
- 建议验证：`go test ./internal/... ./cmd/api -count=1`。这不包含已知有重复 main 的 tmp 包。
- 尚未解决：修改 GORM 标签不会移除已有数据库索引，也不会补已有 Outbox 表缺失的 revision 列；本小步仅验证测试库，不宣称旧开发库已兼容。snapshot.go 与 NotificationState 文件保留但暂不接入。
- 下一步：读取用户实际修改和测试结果，再补显式旧库迁移及相应验证，完成 B0 后才进入 MQ 练习。

本文不会自动被每个新聊天读取。新聊天应先打开 `doc/README.md` 或本文件；不要声称已经通过隐藏记忆永久保存计划。

### 教学记录：2026-10-09，B0 旧库兼容

- 已读取源码确认用户完成第一小步：Join 两处调用 enqueueNotice，Outbox 保留 Revision 且移除唯一索引标签。
- 用户报告 `go test ./internal/... ./cmd/api -count=1` 全部通过；助手本轮未重复运行。
- 已向用户澄清：单 Card 入队仅为临时基线。最终一条 Outbox 对应一次事务中某个接收者的一批变化，可含多张 Card；不等于每次发送完整最近 50 条。
- 本轮给出 migrateOutboxSchema 替换代码：不存在则建表，存在且缺 revision 则显式添加 default 0，移除提前建立的 idx_outbox_receiver_revision，补齐 PublishedAt 查询索引；全程在一个事务中，不清空任务。
- 同时给出旧表升级测试：分别模拟缺 revision 和已有过早唯一索引，检查重复迁移、任务内容与发布状态保留，以及同用户多条旧任务写入。
- 本轮代码仅供用户手写，尚未确认写入或运行；不要标记 B0 完成。已有 snapshot 任务如存在，内容保持原样，本次迁移不承诺转换事件格式；正式恢复开发实例前仍需检查遗留任务。

### 教学记录：2026-10-09，助手已实施旧库迁移与测试

- 用户明确授权“测试和表迁移你帮我改吧”，本次直接编辑 internal/group_migration.go 与 internal/notification_outbox_test.go；其他业务教学仍沿用原约定。
- migrateOutboxSchema 已补 revision、移除过早唯一索引并补发布状态索引；在同一事务中处理，不重建表或删除任务。正式引入版本约束时必须同步替换这段临时移除索引的逻辑。
- 新增 TestOutboxLegacyMigration，通过实际 migrateDatabase 启动入口分别升级两种旧表；验证重复迁移、待投递/已投递任务及非零版本保留、同用户后续任务写入和自增 ID。
- 助手实际执行 `go test ./internal/... ./cmd/api -count=1` 全部通过，`git diff --check` 通过。
- 仅测试临时数据库；未启动开发服务或对实际开发数据库执行迁移。实际旧库会在下次正常启动时走迁移。
- B0 的业务测试与旧表兼容部分完成；tmp 包重复 main 和实际开发实例的遗留任务检查仍是独立待处理项。

### 教学记录：2026-10-09，B1 练习位置调整

- 用户明确指定隔离到 `F:/PROJECTS/go1/playground/rabbit/main.go`；B1 不放 note/cmd，不给 note 添加 MQ 依赖。
- 已确认 playground 自带 go.mod，module 为 playground；rabbit/main.go 当前只有 package main，未发现该目录的额外 AGENTS.md。Docker 可用，未列出名称含 rabbit 的现有容器。
- 本次只贴启动本地 RabbitMQ、在 playground 添加 amqp091-go，以及连接/创建持久队列的入门代码；用户手写，尚未执行或验收。后续再讲发布确认、手动 ACK 和退出重投。

### 教学记录：2026-10-12，返回项目，B2 第一小步

- 用户已学习发送/消费、发布确认、手动 ACK/NACK、multiple 和 Return。发送/接收练习已拆到 playground/rabbit/send 与 receive；发布确认与 ACK 前退出重投未完成全部实际验收，不宣称 B1 故障实验全部通过。遵从用户要求先回项目。
- 再次读取 note 源码，当前仍为旧 Card 入队；NotificationState 只有 Revision，尚无完整状态分配逻辑。
- 本小步仅给用户新增 notification.Change 的代码：Revision、Upserts、RemovedIDs、UnreadCount，表示单用户一次事务的一批变化。Upserts 为完整 Card，UnreadCount 为变化后的绝对值；RemovedIDs 是通知 ID，不是申请 ID。
- 同时修正 Inbox 注释，区分完整快照与增量。此结构是增量正文，不是最终 MQ 信封；task_id/user_id/schema_version/kind 仍按计划另行封装。
- 用户尚未写入，助手未修改 Go；没有宣称旧生产链路已切换或版本机制已生效。删除补位、窗口排序和 rebuild 必须在接入前实现，不能单靠 RemovedIDs 认为处理完毕。
- 下一小步：补 NotificationState.UnreadCount、显式迁移及初始化测试，再实现同事务版本分配和增量入队；测试与迁移遵从用户此前允许助手代改的约定。

### 教学记录：2026-10-12，状态迁移已实现并验证

- 用户已手写 Change 与 NotificationState.UnreadCount，本轮读取确认；用户同意助手实现迁移与测试。
- 新增 notification_state_migration.go，并接入 migrateDatabase 通知表迁移之后。不存在则只建状态表；只有 revision 的半成品表显式补 unread_count 并首次回填；补建有通知但无状态的用户，未读统计不限最近 50 条。
- 保留已有 revision。已包含 unread_count 的既有状态不在重复启动时重算；无通知用户允许没有行，由后续事务写入辅助函数惰性创建。
- 新增测试覆盖三种表结构、55 条未读与已读分离、全已读接收者、重复迁移保留状态和原通知、迁移失败时回滚新增列及重试。
- 助手实际执行 `go test ./internal/... ./cmd/api -count=1` 全部通过，`git diff --check` 通过。仅操作临时测试数据库，未运行开发服务。
- 重要过渡限制：旧 Join 仍不维护此状态；现有查询仍使用原实时统计。因此新表还不能作为可信业务读取源。正式切换新写入/读取前必须停止旧写入并一次性校正此过渡期计数，保留版本并明确重建基线，不能把启动迁移误当持续修复机制。
- 下一步给用户事务辅助代码，配套迁移和测试由助手处理；不得直接把所有旧 Outbox revision=0 加唯一约束。

### 教学记录：2026-10-18，增量入队辅助函数待用户手写

- 已核对源码，仍未实现增量入队函数；旧 Join 继续调用 enqueueNotice。
- 给出 internal/notification/change.go 的 EnqueueChange(tx, receiverID, upserts, removedIDs) (Change, error)：要求传入外层业务事务，业务写入及 Card 加载已在该事务完成，单用户一次业务变化调用一次。
- 函数惰性创建状态，在真实变更事务中 COUNT 全部未读并保存绝对值；revision SQL 原子 +1 且限制在 SQLite int64 范围；编码 Change 并保存 notification.change Outbox。不在普通查询、窗口连接或每条发布时 COUNT，符合计划 6.3。
- 目前仅贴代码，未写入或测试，未接入 Join。调用方必须校验接收者、Card 完整性和变化集合，空变化不应调用；辅助函数返回错误必须让外层事务回滚。
- 后续先核对用户代码并补成功/回滚/连续版本/未读数/版本上限测试，再解决唯一约束、MQ 信封及旧链路兼容；不能直接替换 Join 让旧 SSE 无法识别新事件。

### 教学记录：2026-10-18，增量计数已修改并验证

- 用户明确要求在现有代码上直接修改。已修改 notification/change.go，保留函数签名：已有状态按 unreadDelta 原子加减；只有 INSERT 实际创建状态时才 COUNT 当前通知，此结果已含本次业务修改，不再重复加 delta。
- 加入未读数负数/溢出、revision 范围和 int64 最小 delta 检查；错误要求外层事务回滚。未改 Join 调用、MQ 或现有列表接口。
- 新增 notification_change_test.go：覆盖新增未读、更新未读内容、标记已读、更新已读内容、删除未读/已读、初次插入/已读后初始化；通过查询回调断言已有状态不执行通知 COUNT，首次初始化只执行一次。验证状态/Outbox payload 一致。
- 错误测试覆盖计数下溢/上溢、版本上限、最小 int64 delta、Outbox 插入失败，检查业务通知、状态与任务回滚。
- 实际运行 `go test ./internal/... ./cmd/api -count=1` 全部通过，`git diff --check` 通过。
- 仍需接入前停旧写入并校准过渡期计数，业务调用方根据实际旧/新状态和 RowsAffected 计算 delta。不能按 Upserts/RemovedIDs 长度统一计算。当前辅助函数无法识别逻辑错误但仍在数值范围内的 delta。

### 教学记录：2026-10-18，改为注册时初始化通知状态

- 用户授权直接修改，要求用户创建或第一次登录时初始化为 0。选择注册事务初始化，保证未登录用户收到通知前也已有状态；不增加登录时重置计数。
- auth.Repository.Create 将创建用户和 NotificationState 放在同一事务内，初始 revision/unread_count 均为 0。保留账号后缀/邮箱冲突处理；冲突检查使用 tx，避免 SQLite 单连接自等待。
- 状态迁移改为覆盖所有缺状态的 users：无通知补 0，有旧通知按实际未读数初始化；已有完整状态不重置。此迁移仍不是过渡期旧业务写入的持续计数修复。
- EnqueueChange 删除插入状态、首次 COUNT 分支，仅按 delta 更新现有记录；缺状态作为错误回滚，不静默补建。
- 调整原增量测试，增加缺状态回滚、新用户零状态、首次通知计数、注册状态写入失败导致用户回滚、老用户无通知补零；并发注册测试走完整启动迁移并检查八个用户对应八条初始状态。
- 第一轮测试发现旧并发注册 fixture 仅建 users，修正 fixture 后 `go test ./internal/... ./cmd/api -count=1` 全部通过，`git diff --check` 通过。
- 未操作真实开发数据库，未替换 Join 或接入 MQ。后续仍先校准旧计数并处理新旧事件协议切换。
