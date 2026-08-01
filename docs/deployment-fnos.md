# Cloud and media-node deployment

## 1. Secrets and WireGuard

云端与每台 Windows、飞牛或 Linux 节点分别生成 WireGuard 密钥。每台节点只生成一份独立 API Token；数据库密码、管理员密码、TURN 共享密钥、私钥、令牌和 `.env` 不进入 Git。

```bash
cd deploy/cloud
cp .env.example .env
mkdir -p wireguard/wg_confs
cp wg0.conf.example wireguard/wg_confs/wg0.conf
```

```bash
cd deploy/media-node
cp .env.example .env
mkdir -p wireguard posters
```

云端 `wg0.conf` 为每台节点添加独立 Peer。云端 `.env` 的 `DEER_NODE_CREDENTIALS` 使用相同节点名、API Token 和 `base_url`。节点 `.env` 填入唯一 WireGuard 地址、双方公钥、节点私钥、云服务器端点和节点内部地址。WireGuard UDP 端点直接使用服务器 IP，例如 `SERVER_IP:51820`，不经过 Cloudflare。

## 2. Ports and coturn

云端安全组和主机防火墙开放：

- `80/tcp`、`443/tcp`、`443/udp`：网站与 HTTP/3。
- `51820/udp`：WireGuard 控制通道。
- `3478/tcp`、`3478/udp`：TURN 客户端连接。
- `49160-65535/udp`：TURN 媒体 relay 范围。

`DEER_TURN_EXTERNAL_IP` 填服务器实际持有的公网 IP，coturn 的监听地址和 relay 地址都会只绑定该 IP；多公网 IP 主机不得使用全接口监听。`DEER_TURN_URLS` 使用同一公网 IP 同时配置 UDP 与 TCP URL。不要填写经过 Cloudflare 代理的业务域名，标准代理不转发 `3478`。coturn 使用 TURN REST 临时凭据，容器不保存长期用户列表；relay 禁止访问回环、链路本地和 RFC1918 私网地址，避免利用 TURN 探测云端内网。

管理员默认关闭“允许节点直连”。此时浏览器与节点都只发布 relay candidate，观看者看不到节点公网候选。开启直连后可降低延迟与云端流量，但 WebRTC 对端可能看到节点公网候选。

## 3. Media path

将节点 `.env` 的 `DEER_MEDIA_HOST_PATH` 指向视频根目录。Compose 以 `/media:ro` 挂载，`posters/` 是可重建的封面缓存。

```text
视频根目录/
  工作室甲/
    作品一.mp4
  工作室乙/
    作品二.mp4
```

Windows bind mount 的单个文件名 UTF-8 长度不能超过 255 字节。不能修改原始文件名时运行 `scripts\watch-media-view.cmd`，让节点只读挂载 `E:\BaiduNetdiskDownload\.deer-media-view`；脚本使用同盘硬链接和 `catalog-titles.json` 保留完整标题，不复制或删除原视频。

## 4. Start the cloud stack

独占 `80/443` 的服务器：

```bash
docker compose --env-file .env up -d --build
docker compose ps
docker compose logs --tail=100 wireguard coturn gateway web
```

已有宿主机 Caddy 的服务器：

```bash
docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml up -d --build
curl -fsS http://127.0.0.1:${DEER_WEB_PORT:-28200}/api/v1/health
docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml ps
```

宿主机 Caddy 只增加独立站点块：

```caddyfile
xiaolu.lwylink.xyz {
	encode zstd gzip
	header Strict-Transport-Security "max-age=31536000; includeSubDomains"
	reverse_proxy 127.0.0.1:28200
}
```

替换前备份 `/etc/caddy/Caddyfile`，先运行 `caddy validate --config CANDIDATE`，原子替换后只执行 `systemctl reload caddy`。不得重启或修改服务器上的其他 Compose 项目。

验证 coturn：

```bash
docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml ps coturn
docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml logs --tail=100 coturn
ss -lntup | grep ':3478'
ss -lnup | grep ':51820'
```

## 5. Start a media node

```bash
docker compose --env-file .env up -d --build
docker compose ps
docker compose logs --tail=100 wireguard media-node
```

两端 `wg show` 应有最新握手，管理员节点页应在 90 秒内显示在线。首次扫描逐个执行 `ffprobe` 与封面抽取；目录版本未变化时只发送心跳。媒体节点不再提供 HTTP 视频端点，也没有 Relay Token、带宽令牌桶或连接槽位。

## 6. Playback acceptance

用管理员已解锁视频分别验证两种模式：

1. 关闭节点直连，播放页显示“TURN 隐私中继”，浏览器 `getStats()` 的选中 candidate pair 包含 relay candidate。
2. 开启节点直连，网络允许时显示“节点直连”；NAT 穿透失败时仍自动回落到 TURN。
3. 播放画面与音频实际加载，倍速和全屏可用；Gateway 网络流量不再包含视频字节。
4. 320px、390px、430px 下目录为两列，搜索按钮可见，播放页无横向溢出，后台表格只在表格内滚动。

## 7. Rollback

- Caddy：恢复部署前备份并 reload。
- 云端：仅停止 `deer-screening-room-cloud`，命令需同时带基础与宿主机 Caddy 覆盖文件，不加 `-v`。
- 节点：仅停止 `deer-screening-room-media-1`，保留 WireGuard、封面缓存、兼容视图和原视频。
- WebRTC 版本：恢复部署前 Git 提交、Compose 与 `.env` 备份后重建 Gateway、Web、coturn 和媒体节点。旧 HTTP Relay 代码已删除，回滚必须使用完整旧提交，不能混用新旧 Gateway 与节点。
