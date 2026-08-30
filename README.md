# Note

一个本地优先的 Windows 日历待办应用。用每日清单安排事情，在月历中查看周期日程，并在到点时通过系统通知或独立弹窗提醒。

<p align="center">
  <img src="desktop/build/icon.svg" alt="Note 图标" width="88">
</p>

<p align="center">
  <a href="https://github.com/GH-ytym/note/releases/latest">下载最新版</a>
</p>

## 下载安装

当前提供 Windows x64 版本：

| 版本 | 适合场景 | 下载 |
| --- | --- | --- |
| 安装版 | 安装到电脑，并创建桌面与开始菜单快捷方式 | [Note Setup 0.2.0.exe](https://github.com/GH-ytym/note/releases/download/v0.2.0/Note%20Setup%200.2.0.exe) |
| 便携版 | 不安装，下载后直接运行 | [Note 0.2.0.exe](https://github.com/GH-ytym/note/releases/download/v0.2.0/Note%200.2.0.exe) |

安装包已经包含界面、Go 后端和 SQLite 支持，使用者不需要另行安装 Go、Node.js、数据库或其他运行环境。

> 当前安装包没有代码签名。Windows SmartScreen 可能显示“Windows 已保护你的电脑”，确认文件来自本仓库后，可选择“更多信息” → “仍要运行”。

## 功能

- **每日清单**：按日期查看待办，并以“未完成 / 已完成”分区；标题和时间可以直接修改。
- **月历视图**：查看每一天的日程与颜色标记，支持切换月份和打开指定日期。
- **重复日程**：支持仅一次、每天、工作日、周末、每周、每月，以及选择多个自定义日期。
- **完成状态**：可以只完成当天这一项，也可以把整个周期日程标记为全部完成。
- **两种提醒**：静默提醒进入 Windows 通知中心；弹窗提醒会打开置顶的独立窗口。
- **标题与内容**：标题必填且不可重复；内容可以不填，留空时会自动使用标题。
- **日程颜色**：可以随机生成颜色，也可以手动选择。
- **外观设置**：可以调整背景颜色、主题颜色和整个窗口的不透明度。
- **独立编辑窗口**：日程详情中的内容可以放到更大的聚焦窗口中编辑，保存后回到详情页。
- **系统托盘**：隐藏主窗口后应用仍可在后台运行，提醒和本地 API 会继续工作。

### 提醒规则

应用启动后会安排**今天尚未到点、尚未完成且开启了提醒**的日程。已经过点、当天已完成或全部完成的日程不会提醒；应用完全退出时也不会提醒。

静默提醒本身不播放声音，通知会出现在 Windows 通知横幅或通知中心；弹窗提醒会主动聚焦并闪烁任务栏图标。

## 界面

<table>
  <tr>
    <td align="center"><strong>每日清单</strong></td>
    <td align="center"><strong>新建日程</strong></td>
  </tr>
  <tr>
    <td><img src="docs/images/daily-todos.jpg" alt="每日待办清单"></td>
    <td><img src="docs/images/create-todo.jpg" alt="新建日程窗口"></td>
  </tr>
</table>

### 月历

![Note 月历](docs/images/calendar.jpg)

## 数据与隐私

Note 没有账户、云同步、广告或多人协作功能。日程只保存在本机 SQLite 数据库中：

```text
%APPDATA%\note-desktop\data\note.db
```

外观设置保存在同一目录上一级的 `appearance.json`。备份或迁移时，先从系统托盘中完全退出 Note，再复制整个 `%APPDATA%\note-desktop` 文件夹。

## 技术栈

- 后端：Go、Gin、GORM
- 数据库：SQLite（已启用 WAL、外键和写入等待）
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

该命令会自动构建 React 前端、编译 Go 后端，并启动后端子进程与 Electron 窗口。开发版的数据库同样保存在 Electron 用户数据目录下。

## 浏览器开发模式

先从项目根目录启动 API：

```powershell
go run ./cmd/api
```

首次启动会自动创建 `data/note.db` 并迁移表结构。API 默认只监听本机：<http://127.0.0.1:8080/ping>。

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

- `todos`：日程标题、内容、规则、时间、颜色、提醒方式和版本。
- `todo_dates`：自定义重复模式选择的日期。
- `todo_completions`：只记录已经完成的单次日程日期；未记录即视为未完成。

周期日程的出现时间由查询范围即时计算，数据库不会为未来每一天预先插入记录。

## 构建 Windows 桌面程序

在 `desktop` 目录执行：

```powershell
# 生成可直接运行的目录
npm run pack

# 生成安装版和便携版
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
npm test
node --check main.cjs
node --check preload.cjs
```
