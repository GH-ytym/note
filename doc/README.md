# 开发文档

面向使用者的功能与下载见 [项目 README](../README.md)，部署说明见 [公有云部署指南](../deploy/cloud/README.md)。

本目录保存通知和日历同步的设计计划、实现记录。历史章节可能包含已弃用方案，不能当作当前接口或产品功能说明。

- [通知后端维护计划](notification-backend-maintainer-plan.md)：按最新日期的执行基线阅读。
- [最初通知改造计划](realtime-notification-refactor-plan.md)：保留的入门与历史设计材料。
- [v1.0.0 发布说明](../docs/releases/v1.0.0.md)：本次交付与已知边界。

继续开发时，以实际源码、测试及最新修订为准。当前通知恢复使用数据库同步，不使用 Redis 历史补发；日历通过变更推送与前端低频刷新校正。
