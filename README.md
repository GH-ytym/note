# note

本地备忘录：Go + Gin + GORM + SQLite + React。

SQLite 已嵌入 Go 程序，不需要安装 PostgreSQL、Docker 或 SQLite。

```powershell
cd F:\PROJECTS\go1\note
go run ./cmd/api
```

首次启动会自动创建 `data/note.db` 并同步表结构。API 默认只监听本机：<http://127.0.0.1:8080/ping>。

可以用环境变量修改数据库文件和监听地址：

```powershell
$env:NOTE_DB_PATH = "D:\NoteData\note.db"
$env:HTTP_ADDR = "127.0.0.1:18080"
go run ./cmd/api
```

## 启动 React 前端

保持后端运行，再打开一个终端：

```powershell
cd F:\PROJECTS\go1\note\web
npm install
npm run dev
```

然后访问终端显示的地址，默认是 <http://localhost:5173>。开发服务器会把 `/api` 请求代理到 `127.0.0.1:8080`。

在运行 API 的终端按 `Ctrl+C`，程序会先关闭 HTTP 服务，再关闭数据库连接池。
