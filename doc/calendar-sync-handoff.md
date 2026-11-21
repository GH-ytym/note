# 日历实时同步：当前阶段（2026-11-20）

## 已接入后端

- `calendarsync.EnqueueCalendar`：每个接收者一条 Outbox，名称 `cldr`，Revision 为 0；单条变化包含 `group_id`、`kind`（todo/event）、`id`、`action`（upsert/remove）。
- Todo/Event 创建、修改、删除：在业务事务内给当前群成员写任务。Todo 完成/取消完成也发送 upsert，以更新自己的完成状态及群成员可见的完成名单；重复设置相同完成状态不写任务。
- Todo/Event 权限变化：只给目标用户和创建者写任务，接收者去重。所有群成员仍可查看记录，权限只影响编辑能力。
- Outbox 写入失败时业务事务回滚；业务校验或乐观锁冲突不会生成任务。
- Worker 根据 Name 分流：通知继续 ApplyChange/Rebuild，日历直接 Redis Publish。
- app 在启动 Worker 前确认两种订阅，共用 Hub；SSE 直接转发 cldr，不改变通知 revision。
- 退出时等待 Worker 和两个订阅循环结束；日历订阅建立失败时关闭已经建立的通知订阅。

示例 SSE：

```text
event:cldr
data:{"group_id":3,"kind":"todo","id":42,"action":"upsert"}

```

Publish 成功仅代表 Redis 接受，不代表客户端已收到。重复投递允许发生，前端必须按 kind + id 替换，不能重复追加。成员变动没有在本阶段接入日历推送；查询接口始终重新验证成员资格。

## 前端接收与低频校正（2026-11-20 已实现）

1. 现有 SSE 解析器识别 cldr；useNotifications 将它交给本窗口日历事件总线，不进入通知 revision 逻辑，也不通过桌面 IPC/BroadcastChannel 重复广播。
2. 当前采用范围刷新：相关群的 upsert/remove 都重新查询当前日期范围，成功后整体替换，以覆盖删除、跨日期移动、重复日程和完成记录变化。尚未实现按单条 ID 的实例查询与增量替换。
3. useCalendarItems 统一管理刷新：活跃且可见、联网的日历在请求完成 60 秒后进行下一次校正。隐藏标签、隐藏窗口或离线时暂停；恢复可见、切回日历、恢复网络、窗口聚焦、SSE 首次快照/重连时立即刷新。
4. createCalendarSync 保证当前范围最多一个请求。查询期间的新触发合并为后续一次查询；切群、切日期或卸载时停止旧调度器并取消请求，忽略旧响应。单次查询超时 15 秒。
5. 后台校正保留原数据，不重新打开 loading；成功替换当前范围，失败保留原数据并提示。切换查询范围时才清空旧范围数据并显示首次加载状态。
6. Workspace 把日历标签活跃状态传给 CalendarWindow；小窗日历也正常参与校正。没有选中群时不请求。

2026-11-20 查询优化：GET /calendar 的 group_id 已贯穿 Service 和 Repository，在 Todo/Event SQL 查询中筛选目标群，同时保留当前成员资格检查。完成记录仅查询筛选后的 Todo ID；重复日程只展开筛选后的候选。不传 group_id（或传 0）仍查询用户所属全部群。非成员、退群或不存在的群返回空列表，保持原接口行为。

## 验证

- 真实 app + SQLite + Redis 测试服务 + SSE：通知与日历同时转发、多窗口一致、用户隔离、退出清理。
- Todo/Event 各写操作：任务内容、收件人、Outbox 失败后的业务回滚。
- Worker 混合任务投递、Redis 错误不标记完成及后续重试。
- Stream 在快照查询期间收到日历变化会排队发送，随后通知仍按原 revision 连续处理。

前端自动刷新与定时校正已实现；本阶段没有部署服务器或重新打包 Electron。

验证结果：本次定向测试通过。独立提交内容的 `go test ./...` 仅 `TestEventGroupJoinQuitAndDismiss` 因群解散外键约束失败；在修改前 HEAD 上运行同一测试也失败，属于已有问题。

前端验证：`npm test --prefix web` 44 项通过，包含 SSE 日历事件解析、并发触发合并、查询期间再次变化、60 秒校正、隐藏暂停/恢复、失败保留数据和后续删除校正、切换范围后忽略旧响应、慢请求不重叠。`npm run build --prefix web`（TypeScript 检查与 Vite 构建）通过。
