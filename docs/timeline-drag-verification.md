# 0.5.0 时间轴修复验证

日期：2026-09-12。

## 行为

- 周视图及日时间轴共用 Todo 分轨逻辑。相同时间按稳定 ID 顺序分配 0 至 n-1 轨，超出的 Todo 不渲染；下一时间点重新使用轨道。
- 纵向时间轴从左至右分轨，横向时间轴相应从上至下分轨，与 Event 轨道位置一致。
- Todo 拖动调整开始时间；Event 主体或开始边缘调整开始时间，结束边缘调整结束时间，沿用时钟行为。五分钟步进；拖动提示显示目标日期和时间。
- Pointer Capture 保持拖动，Escape 和 pointercancel 取消，拖动完成不触发详情点击；方向键调整五分钟，Shift+方向键调整 Event 结束时间。
- 时钟拖动提示也改为目标日期和时间。
- 日视图默认保持纵轴为时间、横轴为轨道；配置明确说明“横向排列（纵轴为时间）”，避免把轨道排列方向误认为时间轴方向。
- 修复 Event PATCH 混合 UTC/北京时间导致 SQLite 时间区间 CHECK 失败的问题：写入前统一两个端点到北京时间。

## 验证

- 严格 TypeScript/Vite 构建成功；前端 20 项测试、桌面 21 项测试通过；`go test ./...` 通过。
- SQLite 实库测试覆盖混合时区创建、开始及结束时间 PATCH，检查持久化时间及版本。
- 隔离 Electron 用户目录，正式数据库未写入测试数据。
- 5 个同刻 Todo、轨道上限 3：周纵向及日纵向仅渲染 3 个，轨道分别 0/1/2；横向按相同轨道规则排布。
- 实际指针拖动保存：周纵向 Todo 09:00→09:30，Event 结束 12:00→12:30；周横向 Todo 09:30→10:00，Event 开始 10:00→10:30。
- 日纵向 Todo 09:00→09:15，Event 结束 12:30→12:45；日横向 Todo 09:15→09:30。
- 日纵向 Escape 取消后原时间不变，只有主窗口，未误开详情。交互测试期间无 pageerror。

参考：[FullCalendar 拖动与缩放](https://fullcalendar.io/docs/event-dragging-resizing)、[MDN Pointer Capture](https://developer.mozilla.org/en-US/docs/Web/API/Element/setPointerCapture)。
