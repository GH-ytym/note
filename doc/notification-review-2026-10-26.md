# 通知恢复阶段复核（2026-10-26）

依据：当前本地源码、Git 提交、两份通知计划，以及本次查阅的一手文档。不是逐行比对上游源码；已有计划没有指定上游源码 commit 或移植函数。

## 近期变更

| 提交 | 实际内容 |
| --- | --- |
| b76df2b | Join 接 EnqueueChange，全量 LoadSnapshot 与首次 InitSnapshot |
| d38e23c | ApplyChange 更新缓存、历史、广播及版本检查，未完成写入标记 |
| a0d853b | 重建资格获取/释放，v2 订阅器转发 Hub，相关测试 |
| 0bc7cb9 | 条件安装快照、新 epoch/reset/基线；Lua 拆成文件并 embed；按职责拆 Go 文件 |
| be23afd | Rebuild 串联资格、SQL 快照、安装、释放 |
| 本轮工作区 | ApplyRebuild 分类及测试；更新执行基线，不接 Worker |

ApplyChange 对 items/order/history 错误类型、未完成标记、缺失或格式错误的状态字段、卡片与排序数量不一致返回 rebuild；state 类型错误仍为 error，因为 beginRebuild 不支持修复该类型。Redis 执行错误仍为 error，不能自动解释为需要重建。

## 与计划的关系

主线符合 10-19 基线：SQLite Outbox 直接驱动 Redis、全量缓存、增量更新、安全重建、先恢复后业务。取消 RabbitMQ、取消最近 50 条缓存窗口是用户确定的调整，不应恢复旧方案。

一个实现选择与旧正文不同：7.3 原先推荐 singleflight 合并回源，现在使用按用户 Redis token + 5 秒 TTL，竞争失败直接返回，没有共享同一重建结果。这是之前讨论过的选择；token 控制资格，安装仍比较完整旧 state，不能仅依靠 TTL 防止覆盖。资格过期时安装失败是预期结果。

仍未完成的验收项：

- 后续小步已完成：RunRedisSubscriber 接受并校验 notification.reset，原样转发 Hub；真实安装脚本产生的 reset 已通过零版本、多窗口和用户隔离测试。客户端收到后的重新同步仍未完成。
- Delivery 尚无 Deliver；app.go 与 Worker 仍调用旧 RedisPublisher，新应用链路没有闭环。
- ApplyChange 还不验证 epoch/base_id 等恢复元数据；InitSnapshot 仍不建立新恢复基线。
- HLEN/ZCARD 只能检查数量不等；两个 key 同时丢失、等量内容错误仍可能漏检。
- history 目前限 1000 条，尚未实现计划中的按年龄保留、历史连续性恢复校验。
- Redis 分页读取、SSE 初始快照与实时衔接、断线补发、在线漏消息校正尚未完成。

这些属于阶段缺口；不能把模块测试通过描述为可靠通知链路完成。原计划进度标记落后于代码，本轮已更新顶部，下面历史段落仍只作历史记录。

## 参考资料与明显差异

| 计划引用的一手资料 | 借鉴与差异 |
| --- | --- |
| [Zulip events system](https://zulip.readthedocs.io/en/latest/subsystems/events-system.html) | 借鉴初始状态与事件衔接；Zulip 的注册事件队列流程不是本项目 Redis 缓存结构，本项目对应 SSE 衔接尚未实现。 |
| [Telegram updates](https://core.telegram.org/api/updates) | 借鉴版本去重和缺口处理；Telegram 有多种序列和 getDifference 等补差量协议，本项目每用户 revision，服务端缓存缺口选择 SQL 重建。客户端恢复仍待完成，不能据此推断 Telegram 内部存储。 |
| [Centrifugo history/recovery](https://centrifugal.dev/docs/server/history_and_recovery) | 借鉴有界历史、代次、恢复失败边界、在线位置检查；其流位置为 epoch/offset，本项目有业务 revision 与 Redis Stream ID，并自行维护可变通知快照。目前只完成重建基线，尚无等价完整恢复协议。 |
| [ntfy subscribe API](https://docs.ntfy.sh/subscribe/api/) | 借鉴 SSE、since/poll 补收缓存消息；本项目维护可更新/删除的通知及未读数，不能将消息补收直接当作 Inbox 一致性。对应补收接口尚未接通。 |
| [Slack Real-time Messaging](https://slack.engineering/real-time-messaging/) | 借鉴业务、分发、连接管理分工；Slack 文档描述多服务、多地域 WebSocket，本项目单 Go 进程模块加 SSE，无需照搬规模架构。 |
| [RabbitMQ confirms](https://www.rabbitmq.com/docs/confirms)、[DLX](https://www.rabbitmq.com/docs/dlx) | 原方案后台交接依据；当前取消 MQ，不存在 broker confirm/consumer ACK。Outbox 完成也不能解释成浏览器已经应用。 |

原计划还列出 RabbitMQ [reliability](https://www.rabbitmq.com/docs/reliability)、[consumers](https://www.rabbitmq.com/docs/consumers)、[Go tutorial](https://www.rabbitmq.com/tutorials/tutorial-one-go)；这些保留为旧方案学习资料，本次未逐页复核。Telegram 和 Slack 的文档不是可据此声称已读其服务端开源实现的证据。

本次新增的 rebuild 返回值是本仓库的内部调用协议，不是上述项目的统一规定。总体原则相近，存储结构和协议不同是正常的；真正影响可靠性的差距是尚未完成的恢复衔接与有效性检查。

## 验证范围

`go test -p 1 ./internal/notification -count=1` 通过（隔离 miniredis）。涵盖正常增量、重复/缺口、异常分类、检查无副作用、部分写入标记、损坏后调用 Rebuild 再应用等；没有声称本轮运行完整应用测试或真实 Redis 验证。
