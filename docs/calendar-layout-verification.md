# 日历布局修复验证（2026-09-11）

## 实现

- 搜索框移动到年/月/周/日切换右侧，移除输入框内部 focus outline 与阴影。
- 主日历标题栏使用 Pointer Capture 和显式屏幕坐标 IPC 拖动；交互控件不触发拖动。
- 删除旧 event-overflow 浮层及搜索导航自动展开行为。月格超量按钮进入当天视图。
- 搜索展开页通过 portal 覆盖整个日历，提供返回日历按钮，展开时背景 inert。
- 月格上下均分：上半区显示待办，未完成为点加圆环、完成为实心点；下半区显示按时间长度绘制的圆角日程线段。容量随 ResizeObserver 测得高度变化。
- 默认窗口 980×700，最小 680×460。
- 日程先按完整时间区间分配轨道，再逐日裁切；保留已观察到的事项轨道，翻页继续显示时保持位置。月、周、日视图共用分配结果。周/日继续遵守配置的轨道上限。

## 验证结果

- 严格 TypeScript 检查与 Vite 构建成功。
- 前端单元测试 17/17，桌面测试 21/21。
- 隔离 Electron 用户目录测试，未写入正式数据库。
- 真实鼠标拖动主标题栏，窗口从 (926,318) 移至 (1006,368)，与指针位移 (80,50) 一致。
- 跨天日程 9 月 10 日至 14 日各段 data-track 均为 1，跨周保持第二轨。
- 搜索输入框 outline=none，box-shadow=none；位于视图切换控件右侧，间距 8px。
- 展开搜索边界为 (0,0,980,700)，与窗口内容区一致；返回后背景 inert=false。
- 原 event-overflow DOM 数量为零。
- 原最小尺寸 680×460 下无横向页面溢出，日期标签完整；20 条待办显示 3 条及 +17 入口。
- 前端运行期间未捕获 pageerror。

## 方案参考

- [FullCalendar eventOrder](https://fullcalendar.io/docs/eventOrder)：稳定的开始时间、持续时间排序。
- [Electron custom window interactions](https://www.electronjs.org/docs/latest/tutorial/custom-window-interactions)：拖动区域与交互控件的处理。
