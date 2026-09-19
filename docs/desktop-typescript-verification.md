# Desktop TypeScript 迁移（0.5.0）

2026-09-19：桌面端主进程、preload、窗口布局、工作区状态、提醒调度、构建脚本及全部桌面测试迁移为 `.cts`。启用 TypeScript `strict` 和 `noEmitOnError`，没有 `any` 或 `@ts-ignore` 逃逸。

源码编译到忽略跟踪的 `desktop/dist/`，保留 Electron CommonJS 运行方式。入口为 `dist/main.cjs`，preload 保持 sandbox/contextIsolation，运行时只导入 Electron。`desktop/contracts.cts` 供 preload 和前端共享 IPC 类型，主进程补齐窗口扩展字段和状态类型。

开发启动、Windows/macOS 打包和 CI 已更新到新目录。版本号保持 `0.5.0`。除移除始终返回 null 的旧独立日窗口高度处理分支外，运行行为保持不变。

验证：

- `npm run typecheck --prefix desktop`、前端严格 TypeScript/Vite 构建成功。
- 桌面 22 项测试、前端 20 项测试及 `go test ./...` 通过。
- 新增沙箱 preload 编译产物测试，覆盖 bridge 暴露、IPC 传参及监听器注销；路径测试检查新的开发后端/前端绝对路径。
- 隔离开发版验证 bridge、Go ping、设置窗口及主窗口移动 (30,20)。
- Windows 打包完成，隔离安装内容启动确认 `app.isPackaged=true`、版本 `0.5.0`、Go ping 200。
- 安装内容日期选择完整验证：新建窗口发起 2026-09-19 选择，主日历选择 2026-09-20，返回相同 sessionId/sourceKey，结束后状态为 null，无 pageerror。
- ASAR 检查确认入口为 `dist/main.cjs`，只包含编译后运行模块、图标和 package.json，没有测试文件。

运行时测试使用临时用户目录，未改写正式用户数据库。
