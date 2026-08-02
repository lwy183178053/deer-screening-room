# Cloud and FNOS deployment

## 1. Prepare keys and secrets

在云端与每台 Windows、飞牛或 Linux NAS 节点安装 Docker。节点凭据由管理员页面按节点生成并封装到一次性安装包；数据库密码、管理员密码、私钥、令牌和 `.env` 不进入 Git。

云端复制：

```bash
cd deploy/cloud
cp .env.example .env
mkdir -p wireguard/wg_confs
cp wg0.conf.example wireguard/wg_confs/wg0.conf
```

先在管理员页面创建节点并下载一次性安装包。安装包内固定该节点的 WireGuard 地址、公钥、私钥和 API/Relay Token；删除或轮换节点后旧安装包立即失效。云端 WireGuard Peer 由 Gateway 自动应用，不再维护静态节点凭据映射。云安全组只需开放 `80/tcp`、`443/tcp`、`443/udp` 和 `51820/udp`；不要开放节点的 `8081`。

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

### Use the GHCR image on a NAS

Intel N100 is `linux/amd64`, so the media-node image published by GitHub Actions can be pulled directly on the NAS. The repository publishes `ghcr.io/lwy183178053/deer-screening-room` when a `v*` tag is pushed; the `latest` tag is updated at the same time.

For a private GHCR package, create a GitHub token with `read:packages` and sign in through the NAS Docker registry page or its terminal. Keep the token in the NAS credential store rather than `.env`:

```bash
echo "$GHCR_READ_TOKEN" | docker login ghcr.io -u lwy183178053 --password-stdin
```

Set the selected image tag in the generated `node.env`, import the generated `compose.yaml` as the Docker project, and set only `DEER_MEDIA_HOST_PATH` to the NAS video directory in the project's environment editor. The file has no local `build` step and reads the node identity from the bundle:

```bash
DEER_MEDIA_HOST_PATH=/vol1/1000/video
docker compose --env-file node.env -f compose.yaml pull
docker compose --env-file node.env -f compose.yaml up -d --no-build
```

The image contains both `/app/media-node` and `/app/gateway`; this project starts only `/app/media-node`. WireGuard, node credentials and poster cache remain inside this node project. For another node, create and download a separate bundle instead of copying an existing identity.

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

确认云端 `wg0` 为 `10.77.0.1/24` 后启动节点。节点 WireGuard 日志应显示握手，云端管理员节点页应在 90 秒内显示在线。首次扫描会逐个执行 `ffprobe` 和单任务封面抽取；后续扫描只处理变化文件，目录版本不变时只发送心跳。

## 4. Bandwidth

媒体节点和 Gateway 不再设置视频传输带宽或媒体连接数量上限，数据按实际网络、磁盘和 WireGuard 能力发送。Gateway 仍保留账号级播放请求频率保护，避免异常的 Range 请求洪泛；管理员后台不再提供播放速率设置。
