# 通知 SSE 前端交接

更新日期为 2026-11-14，以当前后端实现为准。

## 连接与消息

通过项目现有鉴权方式连接 `GET /api/notifications/stream`，携带 Bearer token。原生 EventSource 无法自行设置 Authorization 请求头，前端需沿用支持鉴权请求头的流读取方案。token 到期会关闭连接，刷新凭证后重新连接。重连采用退避，避免服务故障时频繁查库。

每次连接首条业务消息是 `notification.snapshot`。服务端已登记该窗口的 Hub 队列，再查询 SQLite 第一页。无需等待 ready，也无需另发全量 snapshot 请求。

```text
event: notification.snapshot
data: {"revision":"12","unread_count":38,"items":[],"next":981}

```

上面的 items 为结构示意，实际包含最多 20 张完整 Card。next 为本页最后一张卡片 ID，有后续页时用于 before，为 0 表示查询时无下一页。unread_count 是该用户全部未读数，与当前加载的卡片数量无关。

后续连续变化使用 `notification.change`，正文包含 revision、upserts、removed_ids 和 unread_count。upserts 携带完整 Card，按 ID 合并；removed_ids 从本地移除；未读角标直接使用绝对 unread_count。单个事件可修改多张卡片。

revision 为十进制字符串，使用 BigInt 等精确方式比较，避免 Number 大整数精度损失。SSE 没有事件 ID，重连每次重新取得第一页快照。服务端每 15 秒发送 SSE 注释心跳，客户端无需把它作为业务事件处理。

## 快照和版本

收到 snapshot 时，用其 items 替换本地列表，同时替换 revision、未读数和 next。清除之前加载的旧页，取消旧分页请求或通过本地请求代次忽略迟到响应。即使新旧快照 revision 相同，快照仍可能要求重置分页，须正常处理。

按 SSE 到达顺序串行应用事件，避免异步处理顺序颠倒。客户端可以跳过 revision 已覆盖的 change。正常 change 版本为当前版本加一；客户端如果发现缺口，应重新建立 SSE 获取新快照。HTTP 操作仅返回单张卡片时，不据此推进整个通知面板的 SSE 同步版本。

服务端已处理连接内的重复和缺口。Redis reset、版本跳号，以及定时发现数据库领先，都会让客户端收到新的 snapshot。前端需能在连接期间处理多次快照。

## 加载旧页

现有 `GET /api/notifications?before=981&limit=20` 从 Redis 读取更旧的卡片。响应包含 items、next、revision、unread_count、epoch。分页响应中的 revision 不用于跳过尚未应用的 SSE 事件，epoch 也不作为 SSE 游标。

分页响应到达时，检查请求仍属于当前快照代次，且响应 revision 与当前已应用的 SSE revision 一致。若不一致，丢弃该响应，必要时重新连接以取得新的第一页。Redis 可能暂时落后于 SQLite，旧响应不能覆盖较新的卡片或未读数。接受响应时按 ID 去重，按 ID 倒序显示。

第一页快照只覆盖有限的通知。change 中未知 ID 的 upsert 可能是尚未加载的旧通知被修改，不能直接假定都是新通知。已加载 ID 可直接替换；已加载区间之外的旧 ID 留待后续分页获取。删除、插入可能影响当前页长度，分页合并和滚动位置需联调。当前后端没有提供跨多个 HTTP 请求固定同一数据库快照的协议。

## 恢复与负载

每个连接每 60 秒查询一次数据库版本。落后时重新发送第一页及全部未读数；查询失败会在下一轮再试，因此 60 秒不是故障下的恢复时限。版本检查只读取状态，落后时才读卡片。

查询超时为 5 秒。快照刷新或发送失败、JWT 到期、窗口队列溢出等会导致连接退出，客户端重连取得新快照。Hub 每个窗口队列容量为 32，服务端关闭队列后可先读完缓冲事件再结束响应。

`GET /api/notifications/snapshot` 当前仍是全量 HTTP 快照入口。常规重连通过 SSE 分页快照完成，避免重复全量查询。

## 验收范围

后端已测试快照期间排队、去重、缺口、reset、版本下限、队列溢出、连接取消和定时校正。定时测试使用 Go 虚拟时钟执行生产定时分支。应用测试使用 SQLite 与 miniredis 验证入群和多窗口推送。

前端仍需验证首次连接、断网重连、凭证到期、快照替换、多窗口已读更新、旧分页响应迟到及分页与实时变化并发。当前未完成真实浏览器或部署验收，已读业务尚待实现。
