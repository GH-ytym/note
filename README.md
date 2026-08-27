# note

最小开发环境：Go + Gin + GORM + PostgreSQL。

```powershell
cd F:\PROJECTS\go1\note
docker compose up -d
go run ./cmd/api
```

启动后访问：<http://localhost:8080/ping>

## 启动 React 前端

保持后端运行，再打开一个终端：

```powershell
cd F:\PROJECTS\go1\note\web
npm install
npm run dev
```

然后访问终端显示的地址，默认是 <http://localhost:5173>。开发服务器会把 `/api` 请求代理到 `localhost:8080`。

在运行 API 的终端按 `Ctrl+C`，程序会先关闭 HTTP 服务，再关闭数据库连接池。

停止数据库：

```powershell
docker compose down
```

本机的 5432 端口已被占用，因此这个项目把 PostgreSQL 映射到 `localhost:5433`。
