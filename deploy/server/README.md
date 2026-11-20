# 服务器版 Note

服务器运行一个 Go API、SQLite 和独立 Redis；客户端不运行数据库或后端。
Web 与 API 通过同一个 HTTPS 入口访问，SSE 同样经过该入口。

## 构建

在项目根目录运行 PowerShell：

```powershell
$env:NOTE_WEB_BASE='/p/note/'
./deploy/server/build.ps1
docker build -t note-api:YYYYMMDD-HHMM build/server/release-具体时间
docker save -o build/server/images.tar note-api:YYYYMMDD-HHMM redis:8-alpine
```

将 images.tar、compose.yaml、源码归档 source.tar.gz、release.json 和对应 SHA256SUMS 放入私有 staging。
真实 JWT 密钥由平台初始化脚本在服务器生成，保留在 `/data/projects/note/.env`，不打进镜像或安装包。
`install.sh --check STAGING RELEASE` 校验后，`install.sh --apply STAGING RELEASE` 发布，健康检查通过才切换 current。
既有库升级前先运行平台 Note 备份；回滚代码使用上一版本 compose 和 release.env，涉及不兼容迁移时需要配套恢复 SQLite 备份。

## 平台接口

- 源仓：GH-ytym/note；平台适配：Epawse/gis-platform 的 `infra/services/gis-server-platform/note`。
- 独立账号：note-svc；独立 rootless daemon：`/run/user/$(id -u note-svc)/docker.sock`。
- 数据：daemon 的 `note_sqlite`、`note_redis` 持久卷；不使用宿主共享 Redis，不发布 Redis 端口。
- 项目目录：`/data/projects/note`；镜像与卷：`/data/containers/note-svc`。
- gateway：`172.18.0.1:18094`，仅平台桥接入口。
- HTTPS：`https://files.gis.epawse.xyz/p/note/`，只配置 websecure 路由，域名当前解析到校园网地址。Tailnet 客户端需将该域名解析到服务器 Tailnet 地址 `100.101.86.29`；仅登录 Tailnet 而仍解析到校园网地址时不可达。
- 健康检查：`/api/ping`（进程就绪，不替代数据库、通知链路联调）。
- 首次上线使用全新数据库，不上传开发电脑的用户数据。Compose 使用服务器本地头像存储，无需 COS。

## 安装包

```powershell
cd desktop
$env:NOTE_SERVER_URL='https://files.gis.epawse.xyz/p/note/'
npm run dist:server
```

输出在 `desktop/release/server`。服务器版只带 Electron 和服务器地址，登录/API/SSE 同源。
此版本加载服务器 Web；更新 Web 无需给每人重装，修改桌面主进程才需要重发安装包。
启动时连接失败会显示重试入口，不回退到本地数据库。

## 查看与停止

以管理员执行 `sudo -u note-svc env XDG_RUNTIME_DIR=/run/user/UID DOCKER_HOST=unix:///run/user/UID/docker.sock docker ...`。
发布脚本中的 `compose` 调用展示了两个 env 文件的参数。日志使用 `logs --tail 100 api`，状态使用 `ps`。
停止使用 `stop`，不要 `down -v`；卷含用户数据库与刷新会话。


## 服务器本地头像

Compose 设置 `NOTE_AVATAR_DIR=/data/uploads`，复用挂载到 `/data` 的 `note_sqlite` 持久卷；个人头像保存到该目录的 `avatars/编号/随机文件名.jpg`。不要将数据库目录本身作为图片静态目录。

`NOTE_AVATAR_URL=/p/note/api/media` 是客户端可见地址前缀，需与反向代理部署路径一致。后端同时提供 `/api/media/*file` 和 `/media/*file`，仅允许读取处理后的头像，不提供目录列表。头像 URL 不要求登录，与此前 COS 公共图片一致；上传、更换和删除仍走认证接口。校园网/Tailnet 的入口限制继续由平台控制。

重建容器保留卷即可保留头像。平台 `note-backup.timer` 每日备份 SQLite 和当前引用的本地头像，保留最近 14 套完整备份；每套包含 SHA256 清单并验证数据库完整性。恢复时应停止 Note API，将同一套 note.db 与 uploads 一起恢复，再启动。可用 `sudo python3 /usr/local/libexec/note/backup.py --verify-latest` 检查最新备份。

本地开发可设置（Vite 继续代理 `/api`）：

```powershell
$env:NOTE_AVATAR_DIR = 'data/uploads'
$env:NOTE_AVATAR_URL = '/api/media'
# 沿用现有 Redis/JWT 配置后启动 API
 go run ./cmd/api
```

设置 `NOTE_AVATAR_DIR` 时优先使用本地存储；未设置时沿用 COS 配置。切换存储不会自动搬迁已有 COS 文件，旧 URL 会继续保留。
