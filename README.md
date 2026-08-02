# 小鹿放映室

小鹿放映室是独立部署的视频目录与鹿币观看系统。公网 Gateway 负责账户、兑换码、鹿币、权益、播放会话和 WebRTC 信令，媒体节点只读扫描本地视频，并通过 FFmpeg 与 Pion WebRTC 向浏览器发送媒体。

首页每页显示 20 部作品并保留工作室筛选，手机端固定两列。管理员默认关闭节点直连，此时浏览器和节点只通过云端 coturn 交换媒体，避免向观看者公开节点网络候选；开启后可优先建立浏览器与节点直连，降低云端流量和延迟。每台 Windows、飞牛或 Linux NAS 节点只需映射本地视频目录，并使用唯一节点名、API Token 和 WireGuard 地址。

## 本地启动

```powershell
Copy-Item .env.example .env
# 修改 DEER_MEDIA_HOST_PATH、DEER_TURN_EXTERNAL_IP、DEER_STUN_URLS 和随机密钥
powershell -ExecutionPolicy Bypass -File .\scripts\dev-up.ps1
```

浏览器打开 `http://localhost:8080`。默认本地管理员来自 `.env` 中的 `DEER_BOOTSTRAP_ADMIN` 和 `DEER_BOOTSTRAP_PASSWORD`。

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke.ps1
```

生产部署、飞牛目录挂载、WireGuard、鹿币兑换、备份和验证说明位于 `docs/`。
