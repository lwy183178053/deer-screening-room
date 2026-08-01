# Testing

后端包含验证码 PNG、一次性挑战、过期和注册强制校验测试：

```powershell
go test ./...
$env:TEST_DATABASE_URL='postgres://postgres:test@localhost:55432/deerroom_test?sslmode=disable'
$env:TEST_HTTP_DATABASE_URL='postgres://postgres:test@localhost:55432/deerroom_http_test?sslmode=disable'
$env:TEST_MEDIA_ROOT='E:\BaiduNetdiskDownload'
go test ./...
```

前端：

```powershell
cd web
npm.cmd ci
npm.cmd run build
npm.cmd run test
npm.cmd run test:e2e
npm.cmd audit --audit-level=high
```

前端 E2E 同时检查随机种子分页、首页工作室筛选、Artplayer 桌面/手机布局，以及播放器 DOM 中不存在广告和外部品牌链接。

部署配置与冒烟：

```powershell
docker compose config
docker compose -f deploy/cloud/compose.yaml --env-file deploy/cloud/.env config
docker compose -f deploy/media-node/compose.yaml --env-file deploy/media-node/.env config
powershell -ExecutionPolicy Bypass -File .\scripts\smoke.ps1
```
