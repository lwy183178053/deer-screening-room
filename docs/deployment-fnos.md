# Cloud and FNOS deployment

## 1. Prepare keys and secrets

在云端与每台 Windows、飞牛或 Linux NAS 节点安装 Docker。云端和每台节点分别生成 WireGuard 密钥，并为每台节点生成独立的 API Token 与 Relay Token；数据库密码、管理员密码、私钥、令牌和 `.env` 不进入 Git。

云端复制：

```bash
cd deploy/cloud
cp .env.example .env
mkdir -p wireguard/wg_confs
cp wg0.conf.example wireguard/wg_confs/wg0.conf
```

第一台节点复制：

```bash
cd deploy/media-node
cp .env.example .env
mkdir -p wireguard posters
```

云端 `wg0.conf` 为每台节点添加独立 Peer；云端 `.env` 的 `DEER_NODE_CREDENTIALS` 使用相同节点名、Token 和 `base_url`。节点 `.env` 填入唯一 WireGuard 地址、双方公钥、节点私钥、云服务器端点和节点内部地址，Compose 启动前会自动生成 `wireguard/wg_confs/wg0.conf`。云安全组只需开放 `80/tcp`、`443/tcp`、`443/udp` 和 `51820/udp`；不要开放节点的 `8081`。

## 2. Configure the media path

在宿主机确认视频根目录的真实路径，将它写入节点 `.env` 的 `DEER_MEDIA_HOST_PATH`。Compose 以 `/media:ro` 挂载，应用不会修改视频文件。`posters/` 是可写生成目录，丢失后可重新扫描生成。

源目录约定：

```text
视频根目录/
  工作室甲/
    作品一.mp4
  工作室乙/
    作品二.mp4
```

Windows Docker 节点还需要注意文件名兼容性：Linux 容器读取 Windows bind mount 时，单个文件名的 UTF-8 长度不能超过 255 字节。中文文件名较长时可能导致整个工作室目录返回 `input/output error`，节点扫描会失败。建议视频文件名控制在 240 个 UTF-8 字节以内；如果目录已经存在超长文件名，请先缩短文件名后再点击后台的“重新扫描”。

如果不能修改原始文件名，可在 Windows 节点运行 `scripts\watch-media-view.cmd`。它会在 `E:\BaiduNetdiskDownload\.deer-media-view` 创建同盘硬链接视图，超长文件名使用短别名，原始完整标题写入 `catalog-titles.json`，节点仍会显示完整标题。Docker 的 `DEER_MEDIA_HOST_PATH` 应指向这个视图目录；原始视频不会被复制或删除。开发机使用 `scripts\dev-up.ps1` 启动时会先生成一次兼容视图；持续新增文件时请让 `watch-media-view.cmd` 保持运行。

## 3. Start services

独占 `80/443` 的服务器先启动云端：

```bash
docker compose --env-file .env up -d --build
docker compose ps
docker compose logs -f wireguard gateway
```

服务器已有宿主机 Caddy 时，使用覆盖配置，Web 只监听宿主机回环端口：

```bash
docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml up -d --build
curl -fsS http://127.0.0.1:${DEER_WEB_PORT:-28200}/api/v1/health
```

宿主机 Caddy 为新域名单独增加站点，不改其他站点块：

```caddyfile
video.example.com {
	encode zstd gzip
	header Strict-Transport-Security "max-age=31536000; includeSubDomains"
	reverse_proxy 127.0.0.1:28200
}
```

修改前备份现有 Caddyfile，使用 `caddy validate --config` 验证候选文件后再替换并执行 `systemctl reload caddy`。回滚时恢复备份并 reload；停止小鹿服务时必须同时指定两份 Compose 文件且不要使用 `-v`。

确认云端 `wg0` 为 `10.77.0.1/24` 后启动节点：

```bash
docker compose --env-file .env up -d --build
docker compose ps
docker compose logs -f wireguard media-node
```

节点 WireGuard 日志应显示握手，云端管理员节点页应在 90 秒内显示在线。新增第二节点时使用 `.env.node-2.example` 作为起点，分配新名称、地址、密钥和本地目录，并在云端增加对应 Peer 与凭据映射。首次扫描会逐个执行 `ffprobe` 和单任务封面抽取；后续扫描只处理变化文件，目录版本不变时只发送心跳。

## 4. Bandwidth

节点不再设置总带宽令牌桶，媒体节点会按实际网络和磁盘能力发送数据；仍保留 `DEER_NODE_MAX_STREAMS` 并发连接上限。单账号播放速率由管理员后台“设置”保存，初始为 10 Mbps。
