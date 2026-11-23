# 公有云部署：SQLite + Redis + COS

## 无域名模式：公网 IP + HTTPS

当前可直接使用 `https://124.220.23.245`，无需购买域名。`.env` 中 `NOTE_DOMAIN` 虽沿用旧名称，现在也接受公网 IP。Caddy 固定为已验证的 2.11.9，显式向 Let's Encrypt 申请 `shortlived` 公网证书，配置 `default_sni` 以支持不发送 SNI 的 IP 客户端；不使用自签证书，也不关闭客户端证书校验。

证书有效期 160 小时，Caddy 常驻维护并自动续期，证书和 ACME 账号保存在 `note-cloud_caddy_data` 卷。首次签发及续期要求公网验证端口可达，并能访问 Let's Encrypt。必须保留 TCP 80/443 防火墙规则、持久卷和容器自动重启策略；IP 改变后需要更新配置和客户端地址。自动续期不能代替监控，应定期检查 HTTPS 访问、证书剩余有效期和 Caddy 错误日志。

官方依据：[IP 证书正式发布](https://letsencrypt.org/2026/01/15/6day-and-ip-general-availability)、[Caddy ACME profile](https://caddyserver.com/docs/caddyfile/directives/tls)。

在 API/COS 尚未配置时，可只启动 HTTPS 网关（目录内须有 Caddyfile 和仅含 `NOTE_DOMAIN=公网IP` 的 `.env`）：

```bash
sudo docker compose -f compose.gateway.yaml up -d
curl --fail https://124.220.23.245/healthz
sudo docker compose -f compose.gateway.yaml logs --tail 50
```

`/healthz` 只代表 HTTPS 网关，不代表 Note API 已就绪。此阶段其他路径可能返回 502。补全 COS、JWT、镜像配置后，在相同 `/opt/note` 目录执行 `sudo docker compose -f compose.yaml up -d --wait`，复用现有证书卷；再验证 `/api/ping` 和登录业务。不要执行删除卷的命令。

桌面打包使用 `$env:NOTE_SERVER_URL = 'https://124.220.23.245/'`。下面的域名购买、DNS 配置步骤在 IP 模式下跳过；云厂商对公开服务的要求仍需遵守，IP 证书仅解决 TLS 技术问题。

后端和网页运行在一台 Linux x86_64 云服务器，SQLite 保存在 Docker 的 `note-cloud_sqlite` 卷，Redis 使用独立卷。头像通过现有 COS 实现上传，数据库只保存 URL；本方案不设置 `NOTE_AVATAR_DIR`，不使用服务器头像目录，不需要 MySQL。

## 1. 准备资源

- 腾讯云服务器：Ubuntu 24.04 x86_64，首轮小规模测试可从 2 核 2GB 起步，4GB 更宽裕，实际容量以测试为准。
- 一个域名，将子域名（例如 `note.example.com`）的 DNS A 记录指向服务器公网 IP。只有支持 IPv6 时才设置 AAAA。
- 按 [Docker 官方说明](https://docs.docker.com/engine/install/ubuntu/) 安装 Docker Engine 和 Compose 插件。下面的命令假定当前用户有 Docker 权限。
- 若官方 Docker 下载站无法访问，可运行本目录的 `install-docker.sh`，从 Ubuntu 签名软件源安装 `docker.io` 和 `docker-compose-v2`。2026-11-23 已在新腾讯云实例安装 Docker 29.1.3、Compose 2.40.3，并通过隔离容器运行检查。当前使用 `sudo docker ...`，没有给普通用户添加 Docker 组权限；执行下文命令时相应加 `sudo`，备份可用 `sudo bash backup.sh --apply`。
- 防火墙放行 TCP 80/443，SSH 限管理来源；不要开放 SQLite/API/Redis 端口。Caddy 自动申请和续期 HTTPS 证书，证书保存在卷内。
- 中国大陆公网域名服务上线前确认备案要求：[腾讯云备案文档](https://cloud.tencent.com/document/product/243/18905)。

## 2. 配置头像桶

在 COS 建立专用头像桶，选接近服务器和用户的地域。当前代码返回可直接访问的图片 URL，因此头像对象需要匿名读取：可以给 `avatars/*` 配置匿名 `GetObject`，写入始终私有；不要给匿名上传、删除或列桶权限。这里的头像是公开图片，不适合存放私密附件。

创建专用 CAM 子账号密钥，只授权该桶 `avatars/*` 的 `PutObject` 和 `DeleteObject`。密钥仅填写到服务器 `.env`，不会进入客户端。默认 HTTPS 桶域名可直接作为 `NOTE_COS_BUCKET_URL`；`NOTE_COS_PUBLIC_URL` 留空即可，如以后用自定义域名再填写。

上传流程：客户端 → 后端校验并压缩图片 → COS → SQLite 保存 URL → 客户端直接读取图片。旧头像在新头像保存成功后删除。

在控制台开启 COS 版本控制，保留误删除恢复能力；它不是独立备份。如需要更强容灾，另设私有备份桶和复制策略。版本保留会增加存储费用，生命周期保留期至少覆盖数据库备份恢复窗口。恢复旧数据库后，对应的旧头像可能需要去除删除标记才能再次访问。

参考：[桶域名](https://cloud.tencent.com/document/product/436/6224)、[版本控制](https://cloud.tencent.com/document/product/436/19883)。这些云端设置需要实际创建资源，本仓库不会自动配置云账号。

## 3. 本机打包后端

在项目根目录用 PowerShell 执行（需启动 Docker Desktop）：

```powershell
./deploy/cloud/build.ps1
```

脚本将网页按根路径 `/` 构建，编译 Linux 后端，生成 Docker 镜像，并导出到 `build/cloud/cloud-时间戳/`。将输出目录内的全部文件（包括 `.env.example`）上传到服务器固定目录，例如 `/opt/note`。后续发布也使用这个目录和相同 Compose 项目名。

## 4. 服务器启动

```bash
cd /opt/note
sha256sum -c SHA256SUMS
docker load -i images.tar
cp .env.example .env
chmod 600 .env
cat image.txt
openssl rand -hex 32
nano .env
```

填写域名（不带协议和路径），将 `NOTE_IMAGE` 改成 `image.txt` 内容，JWT 密钥改成生成的随机值，填写 COS 桶地址及子账号密钥。密钥以后升级时保持不变；示例值不可直接用于上线。

```bash
docker compose config --quiet
docker compose pull redis caddy
docker compose up -d --wait --wait-timeout 120
docker compose ps
curl --fail https://你的域名/api/ping
docker compose logs --tail 100 api caddy
```

`--wait` 检查容器健康，HTTPS 仍需用 curl 单独验证。首次启动创建空数据库。访问网页注册账号，上传、更换、删除头像，再用另一个账号检查头像和通知显示。没有真实 COS 凭据时，本地测试不能替代这一步。

## 5. 打包桌面客户端

在本机项目根目录：

```powershell
$env:NOTE_SERVER_URL = 'https://你的域名/'
npm run dist:server --prefix desktop
```

安装包在 `desktop/release/server/`。旧服务器版安装包仍指向课题组服务器，需要换装这次的新包；以后域名不变时，仅更新服务端网页通常不需要重新发包。

## 6. 备份、更新和恢复

```bash
cd /opt/note
bash backup.sh --check
bash backup.sh --apply
```

备份会短暂停止 API，复制完整 SQLite 数据目录和 `.env` 后重启，输出带校验值的压缩包。文件含密钥，只能私下保存；应复制到服务器外的私有备份位置。COS 图片不在这个备份内，Redis 会话也不备份。脚本不会自动清理或自动上传备份，也没有自动安装定时任务。

更新：先备份，上传新发布文件但保留 `.env`，`docker load -i images.tar`，把 `.env` 的 `NOTE_IMAGE` 更新为新 `image.txt`，再执行 `docker compose up -d --wait --wait-timeout 120` 并验证健康和业务。不要执行 `docker compose down -v`，它会删除持久数据卷。

回滚程序：将 `NOTE_IMAGE` 改回之前保留的镜像，再启动；如果升级改了数据库结构，必须先确认旧版兼容，不能仅换镜像。数据库恢复应停止 API，先保留故障现场，再将已校验备份恢复到独立测试卷验证，确认后替换正式数据；不要直接覆盖运行中的 SQLite 文件。恢复 `.env` 时注意当前域名和 COS 凭据。

## 7. 旧服务器数据

本方案不自动迁移课题组服务器数据，也不会停止旧服务。若保留测试账号和群组，需要停写后搬迁一致的 SQLite 备份。已有本地头像 URL 不会自动变成 COS URL：还需要上传旧头像到 COS、更新数据库中的头像引用，并检查通知快照等引用位置；或者由用户在新服务重新上传头像。完成迁移核验前保留旧数据和备份。

## v1.0.0 云端验收记录（2026-11-23）

API、Redis 和 HTTPS 网关已在腾讯云运行。头像密钥存放在服务器权限为 `600` 的 `.env` 中，不包含在源码或客户端内。头像目录只开放匿名 `GetObject`；服务子用户只具有该目录的上传、删除权限。

已实测上传、更换、移除头像；匿名图片请求返回 200，网页个人设置中正常显示 512×512 JPEG。COS 默认域名返回 `Content-Disposition: attachment`，本次浏览器内 `<img>` 加载仍正常；直接打开图片链接可能触发下载。

首次部署创建的是独立空数据库，旧课题组服务器数据没有迁移。SQLite 备份脚本已经提供，但本次没有配置定时备份任务；COS 版本控制已开启，仍需按恢复窗口规划独立备份和保留策略。
