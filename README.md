# Note

一个以日历为核心的本地桌面备忘录，使用 Go 提供 API 与数据存储，React 构建界面，并由 Electron 打包成独立桌面程序。

数据保存在本机 SQLite 文件中。使用打包后的桌面版时，不需要另行安装 Go、Node.js、Docker、PostgreSQL 或 SQLite。

## 当前功能

- 创建、查看、修改和删除日程。
- 日历按时间范围查询日程，不会提前生成无限数量的周期记录。
- 支持仅一次、每天、工作日、周末、每周、每月和自定义日期。
- 支持随机颜色和自选颜色。
- 支持单次日程完成状态，以及“全部完成”开关。
- 当天日程按“未完成 / 已完成”分区，并可直接修改内容与时间。
- Electron 多窗口：当天日程主窗口、月历、新建日程和日程详情相互独立。
- 主窗口关闭后进入系统托盘，Go 服务和数据库连接会随应用退出而关闭。

静默提醒和弹窗提醒目前只会保存为日程配置，真正的系统通知调度尚未实现。

## 技术栈

- 后端：Go、Gin、GORM
- 数据库：SQLite（WAL、外键和写入等待已启用）
- 周期计算：`rrule-go`
- 前端：React、Vite、Phosphor Icons
- 桌面端：Electron、electron-builder

## 项目结构

```text
note/
├─ cmd/api/                 Go 程序入口
├─ internal/
│  ├─ handler/              HTTP 参数解析与响应
│  ├─ model/                GORM 数据模型
│  ├─ router/               Gin 路由
│  ├─ todo/                 Todo 业务与数据访问
│  ├─ retry/                通用重试
│  └─ app.go                数据库、HTTP 服务与优雅关闭
├─ web/                     React 前端
└─ desktop/                 Electron 主进程、预加载脚本与打包配置
```

## 桌面版开发运行

开发环境需要：

- Go 1.26 或更高版本
- Node.js 和 npm
- Windows x64（当前 Electron 打包目标）

第一次运行先安装两部分依赖：

```powershell
cd web
npm install

cd ..\desktop
npm install
```

然后从 `desktop` 目录启动：

```powershell
npm run start
```

该命令会自动完成以下工作：

1. 构建 React 前端。
2. 编译 Go 后端。
3. 启动 Go 子进程和 Electron 窗口。
4. 在 Electron 的用户数据目录中创建 `data/note.db`。

## 浏览器开发模式

先从项目根目录启动 API：

```powershell
go run ./cmd/api
```

首次启动会自动创建 `data/note.db` 并同步表结构。API 默认只监听本机：<http://127.0.0.1:8080/ping>。

再打开一个终端启动前端：

```powershell
cd web
npm run dev
```

访问终端显示的地址，默认是 <http://localhost:5173>。Vite 会把 `/api` 请求代理到 `127.0.0.1:8080`。

在 API 终端按 `Ctrl+C` 后，程序会先优雅关闭 HTTP 服务，再关闭 SQLite 连接池。

## 环境变量

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `NOTE_DB_PATH` | `data/note.db` | SQLite 数据库文件位置 |
| `HTTP_ADDR` | `127.0.0.1:8080` | HTTP 监听地址；端口设为 `0` 时由系统分配 |
| `NOTE_WEB_DIR` | 空 | 可选的 React 构建产物目录，Electron 会自动设置 |
| `NOTE_STOP_ON_STDIN_CLOSE` | 空 | 设为 `1` 时，父进程关闭 stdin 后停止服务；由 Electron 使用 |

示例：

```powershell
$env:NOTE_DB_PATH = "D:\NoteData\note.db"
$env:HTTP_ADDR = "127.0.0.1:18080"
go run ./cmd/api
```

## API 概览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/ping` | 健康检查 |
| `POST` | `/api/todos` | 创建日程 |
| `GET` | `/api/todos` | 分页查询 Todo |
| `GET` | `/api/todos/:id` | 查询单个 Todo |
| `PATCH` | `/api/todos/:id` | 修改 Todo，使用 `version` 乐观锁 |
| `DELETE` | `/api/todos/:id` | 删除 Todo |
| `GET` | `/api/calendar?from=YYYY-MM-DD&to=YYYY-MM-DD` | 查询时间范围内的日程实例 |
| `PATCH` | `/api/todos/:id/occurrences/:date` | 修改某个 Todo 在某一天的完成状态 |

## 数据表

- `todos`：日程规则、内容、时间、颜色、提醒方式和版本。
- `todo_dates`：自定义重复模式选择的日期。
- `todo_completions`：只记录已经完成的单次日程日期；未记录即视为未完成。

周期日程的出现时间由查询范围即时计算，数据库不会为未来每一天预先插入记录。

## 构建 Windows 桌面程序

在 `desktop` 目录执行：

```powershell
# 生成可直接运行的目录
npm run pack

# 生成安装包和便携版
npm run dist
```

构建结果位于 `desktop/release/`。打包过程会自动重新构建 React 前端和 Go 后端。

## 检查

```powershell
# 项目根目录
go test ./...

cd web
npm run build

cd ..\desktop
node --check main.cjs
node --check preload.cjs
```

## 当前定位

这是一个单机、本地优先的日历备忘录，目前没有账户、多人协作和远程同步功能。SQLite 数据文件就是用户的主要数据，请在需要时自行备份。
