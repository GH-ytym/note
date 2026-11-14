// Package notification 实现通知的事务记录、Redis 缓存、Outbox 投递和在线转发。
//
// 写入链路：业务事务修改通知后调用 EnqueueChange，把版本、未读数和 Outbox
// 一起提交到数据库。RunOutboxWorker 轮询任务，Delivery.Deliver 先尝试
// RedisCache.ApplyChange；缓存缺失、不完整或版本断档时，由 Rebuilder.Rebuild
// 从数据库加载完整快照。投递完成后，Worker 才标记 Outbox 已发布。
//
// 读取链路：Service.List 调用 Reader.List，优先用 RedisCache.ReadPage 读一页。
// 缓存不可用时尝试一次重建，再读取一次；不会把完整数据库快照直接当作一页返回。
//
// 推送链路：缓存写入脚本广播 live，RunRedisSubscriber 把事件
// 交给 Hub 中该用户的各个连接，再由 SSE handler 写给浏览器。
package notification
