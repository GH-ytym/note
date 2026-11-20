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

## 下一阶段：前端接收与低频校正（尚未实现）

1. 在现有通知 SSE 解析中识别 cldr，保持通知版本逻辑独立。
2. remove 删除该 kind + id 的全部本地实例；upsert 重新查询该记录在当前范围的实例，先移除旧实例再替换。现有 calendar 接口尚无按记录 ID 查询能力，不能只取详情当作重复日程实例，需要补这个查询能力或先刷新当前范围。
3. `web/src/hooks/useCalendarItems.ts` 统一管理刷新：当前可见、活跃日历每 60 秒校正；切换范围/群、恢复网络、重新激活、SSE 重连时立即刷新。
4. 同时最多一个当前范围请求。期间收到刷新触发时标记 pending，完成后再查；旧范围结果通过 generation 丢弃。定时查询完成后再计时，避免慢请求重叠。
5. 后台校正不清空已有数据、不展示全屏 loading；成功时替换当前范围，失败保留数据并等待下次触发。
6. 当前 hook 已有 generation 防旧响应覆盖，但还没有定时器、单请求合并或 SSE cldr 接入。还需结合 CalendarWindow 的活跃标签状态，不能只依赖 document.visibilityState 判断。

注意：现有 GET /calendar 虽然接收 group_id，后端是在 Handler 最后过滤群；SQL 仍查询用户所属群的范围候选。不能声称 SQL 已只查当前群。启用周期查询前，应把 group_id 下推到查询层，并保持群成员资格检查。

## 验证

- 真实 app + SQLite + Redis 测试服务 + SSE：通知与日历同时转发、多窗口一致、用户隔离、退出清理。
- Todo/Event 各写操作：任务内容、收件人、Outbox 失败后的业务回滚。
- Worker 混合任务投递、Redis 错误不标记完成及后续重试。
- Stream 在快照查询期间收到日历变化会排队发送，随后通知仍按原 revision 连续处理。

本阶段没有部署服务器，也没有实现前端自动刷新或定时校正。

验证结果：本次定向测试通过。独立提交内容的 `go test ./...` 仅 `TestEventGroupJoinQuitAndDismiss` 因群解散外键约束失败；在修改前 HEAD 上运行同一测试也失败，属于已有问题。
