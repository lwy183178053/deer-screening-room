# Testing

后端：

```powershell
go test ./...
go test -race ./...
go vet ./...
$env:TEST_DATABASE_URL='postgres://postgres:test@localhost:55432/deerroom_test?sslmode=disable'
$env:TEST_HTTP_DATABASE_URL='postgres://postgres:test@localhost:55432/deerroom_http_test?sslmode=disable'
go test ./...
```

WebRTC 单元与集成测试覆盖 TURN REST 临时凭据、Gateway 强制 ICE 策略、节点 relay-only 配置、HTTP 请求结束后的会话存活、错误脱敏、重复关闭和旧播放接口 `404`。设置 `TEST_HTTP_DATABASE_URL` 后会执行真实 PostgreSQL 播放会话、offer/answer 与关闭闭环。

前端：

```powershell
cd web
npm.cmd ci
npm.cmd run test
npm.cmd run build
npm.cmd run test:e2e
npm.cmd audit --audit-level=high
```

Vitest 使用 `RTCPeerConnection` mock 验证 offer、answer、relay-only 策略、candidate pair 状态和卸载关闭。Playwright 检查每页 20 条、320/390/430px 双列目录、搜索按钮边界、原生 WebRTC 播放页、后台局部横向滚动、账户与弹窗布局。

部署配置：

```powershell
docker compose --env-file .env.example -f compose.yaml config --quiet
docker compose --env-file deploy/cloud/.env.example -f deploy/cloud/compose.yaml config --quiet
docker compose --env-file deploy/cloud/.env.example -f deploy/cloud/compose.yaml -f deploy/cloud/compose.host-caddy.yaml config --quiet
docker compose --env-file deploy/media-node/.env.example -f deploy/media-node/compose.yaml config --quiet
git diff --check
```

生产验收必须使用真实浏览器和真实媒体，分别验证“TURN 隐私中继”和“节点直连/自动回落”状态，确认画面、音频、倍速、全屏与 candidate pair；仅有 SDP 成功或容器 healthy 不代表播放链路验收通过。
