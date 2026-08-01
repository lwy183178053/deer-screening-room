# 小鹿放映室

小鹿放映室是独立部署的视频目录与鹿币观看系统。公网 Gateway 负责账户、兑换码、鹿币、权益和流量控制，飞牛 NAS 媒体节点只读扫描本地视频并通过 WireGuard 提供 HTTP Range 数据。

首页随机展示各 Docker 媒体节点的视频并保留工作室筛选，独立播放页使用完全自托管、无广告和外部链接的 Artplayer。每台 Windows、飞牛或 Linux NAS 节点只需映射本地视频目录，并使用唯一节点名和 WireGuard 地址。

## 本地启动

```powershell
Copy-Item .env.example .env
# 修改 DEER_MEDIA_HOST_PATH 指向本机视频目录
powershell -ExecutionPolicy Bypass -File .\scripts\dev-up.ps1
```

浏览器打开 `http://localhost:8080`。默认本地管理员来自 `.env` 中的 `DEER_BOOTSTRAP_ADMIN` 和 `DEER_BOOTSTRAP_PASSWORD`。

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke.ps1
```

生产部署、飞牛目录挂载、WireGuard、鹿币兑换、备份和验证说明位于 `docs/`。
