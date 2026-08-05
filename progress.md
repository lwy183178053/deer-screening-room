# Progress

## 2026-08-03 - Task: 更新本机媒体节点到自动视图镜像
### What was done
- 将本机 `deer-node` 更新为公开 GHCR 镜像 `v0.1.1`，保留现有 `deer-wg` 容器、WireGuard 卷和节点身份，仅重建媒体节点。
- 发现 Windows Docker 直接枚举原始长中文目录会返回 `input/output error`，恢复并启动主机 `watch-media-view.ps1`，节点改为读取 `.deer-media-view` 后正常扫描。
- 更新部署文档，明确 Linux/NAS 原生目录使用容器自动视图，Windows Docker 超长目录使用主机监听器视图。

### Testing
- `docker pull ghcr.io/lwy183178053/deer-screening-room:v0.1.1`：成功，摘要为 `sha256:3a66ea94d80e11450d7d1bb212ad515cdd39e2b9e2af122e7310d40508fde615`。
- 本机 `deer-node` 镜像为 `v0.1.1`，容器健康接口返回 `{"service":"media-node","status":"ok"}`。
- WireGuard `deer-wg` 保持 healthy，最新握手持续更新。
- 主机兼容视图包含 79 个视频和标题清单；节点 `/media` 视图包含 79 个可跟随链接及 `catalog-titles.json`。
- ModelRoute、Redis、MySQL 等其他容器状态保持原样。

### Notes
- `deploy/media-node/.env`：本机忽略运行配置更新为 `v0.1.1`、GHCR 镜像和 Windows 兼容视图路径；节点凭据从最后一次本地导出包恢复。
- `docs/deployment-fnos.md`、`docs/media-library.md`：补充 Windows Docker 长目录边界和主机监听器要求。
- 回滚方式：停止当前 `deer` 节点项目并恢复旧节点包的 Compose/环境配置；不要删除原始媒体目录或 WireGuard 卷，`media-view` 卷可单独清理。

## 2026-08-03 - Task: 跨平台媒体兼容视图自动生成
### What was done
- 媒体节点新增跨平台兼容视图同步：源目录只读挂载到 `/source`，节点在独立 `media-view` 卷的 `/media` 生成稳定短别名，优先使用硬链接，跨文件系统时回退为软链接。
- 每次首次、定时或手动扫描前自动同步新增、变更和删除的视频；超长文件名通过 `catalog-titles.json` 恢复完整标题，原始视频不复制、不重命名、不删除。
- 更新 Windows、Linux/NAS 节点 Compose、管理员导出安装包模板和部署文档；用户仍只需设置 `DEER_MEDIA_HOST_PATH`，新版节点包会自动启用视图。

### Testing
- `go test ./...`：通过。
- `go test -race ./...`：通过。
- `go vet ./...`：通过。
- `npm.cmd run test -- --run`：3 个测试文件、6 项通过。
- `npm.cmd run build`：通过。
- `docker compose --env-file deploy/media-node/.env.ugreen.example -f deploy/media-node/compose.ugreen.yaml config --quiet`：通过。
- `docker compose --env-file deploy/media-node/.env.example -f deploy/media-node/compose.yaml -f deploy/media-node/compose.registry.yaml config --quiet`：通过。
- `docker build --tag deerroom-app:media-view-20260803 .`：通过。
- `git diff --check`：通过。
- 回归测试覆盖长文件名稳定别名、完整标题映射、硬链接视图、新文件加入和源文件删除清理。

### Notes
- `internal/media/view.go`、`internal/media/view_test.go`：新增跨平台视图同步器及测试。
- `internal/media/node.go`、`internal/media/scanner_test.go`：扫描前自动同步视图并覆盖节点级回归。
- `internal/config/config.go`、`cmd/media-node/main.go`：读取并传递 `DEER_MEDIA_SOURCE_ROOT`。
- `deploy/media-node/compose.yaml`、`deploy/media-node/compose.ugreen.yaml`、`deploy/media-node/.env.example`、`deploy/media-node/.env.ugreen.example`：新增 `/source` 只读源目录和 `media-view` 可写卷。
- `internal/provisioning/provisioning.go`、`internal/provisioning/provisioning_test.go`：更新新节点 ZIP 的环境、Compose 和说明。
- `docs/architecture.md`、`docs/deployment-fnos.md`、`docs/media-library.md`：记录自动视图行为和目录配置。
- 回滚方式：节点回退到上一版镜像/节点 ZIP 并恢复原来的 `${DEER_MEDIA_HOST_PATH}:/media:ro` 挂载；停止并删除 `media-view` 卷不会影响只读源目录中的原始视频。

## 2026-07-31 - Task: 建立独立后端、账户、鹿币与交易闭环
### What was done
- 初始化“小鹿放映室”独立 Git 项目，建立 Go 网关、PostgreSQL 迁移、邮箱会话、鹿币钱包、易支付充值、兑换码、单片解锁和会员续费能力。
- 通过钱包流水、行锁和唯一业务引用保证充值、兑换与消费幂等，管理员生成的原始兑换码仅在创建时导出。

### Testing
- `go test ./...`：通过。
- `TEST_DATABASE_URL=postgres://postgres:test@localhost:55432/deerroom_test?sslmode=disable go test ./...`：通过真实 PostgreSQL 17 验证重复支付只入账一次、同一兑换码并发仅一人成功、重复解锁不重复扣款、会员按 30 天顺延。

### Notes
- `.gitignore`：排除环境变量、依赖、构建结果与运行数据。
- `.dockerignore`：排除镜像构建无关文件。
- `go.mod`：声明独立 Go 模块及后端依赖。
- `go.sum`：锁定 Go 依赖校验值。
- `cmd/gateway/main.go`：新增网关启动、迁移、管理员引导和优雅关闭。
- `internal/config/config.go`：新增网关、媒体节点、支付与限速配置读取。
- `internal/password/password.go`：新增 Argon2id 密码哈希与校验。
- `internal/password/password_test.go`：验证正确及错误密码行为。
- `internal/bandwidth/manager.go`：新增按账号复用的流式令牌桶。
- `internal/zpay/zpay.go`：新增易支付签名与金额精确解析。
- `internal/zpay/client.go`：新增易支付收银台和主动查单客户端。
- `internal/zpay/zpay_test.go`：验证签名和金额边界。
- `internal/store/migrations/001_initial.sql`：建立账户、钱包、订单、兑换码、媒体、权益和播放会话表。
- `internal/store/types.go`：定义安全输出与业务类型。
- `internal/store/postgres.go`：新增迁移、账户、会话、钱包查询与审计存储。
- `internal/store/commerce.go`：新增充值入账、兑换、解锁、会员和后台查询事务。
- `internal/store/postgres_integration_test.go`：新增真实数据库交易并发与幂等验证。
- `internal/httpapi/router.go`：新增路由、安全响应头、认证与错误边界。
- `internal/httpapi/auth.go`：新增邮箱注册、登录、会话和退出。
- `internal/httpapi/auth_test.go`：验证邮箱与密码输入边界。
- `internal/httpapi/commerce.go`：新增鹿币、会员、易支付和兑换码 HTTP 接口。
- `internal/httpapi/routes_pending.go`：保留下一阶段媒体与目录路由注册点。
- 回滚方式：删除独立目录 `E:\AllProject\小鹿放映室`；现有网盘仓库未被修改。

## 2026-07-31 - Task: 实现媒体扫描、节点同步与鉴权播放
### What was done
- 建立飞牛媒体节点的增量扫描、格式识别、封面抽取、目录编目、心跳与云端同步，真实路径仅保留在 NAS 节点。
- 建立游客目录、管理员视频管理、单账号单播放会话、双层限速和完整 HTTP Range 代理；节点或文件失联时保留视频与用户权益。

### Testing
- `TEST_MEDIA_ROOT=E:\BaiduNetdiskDownload go test ./...`：通过两个真实 1080p H.264/AAC MP4 的探测和封面生成，非视频文件被忽略。
- PostgreSQL 集成测试：通过节点 90 秒在线窗口、播放会话替换、已购权益和目录同步验证。
- HTTP 集成测试：通过 Cookie/CSRF 播放授权、Relay Token、`Range: bytes=2-5` 转发以及 `206 Content-Range` 响应验证。
- `go test ./...`：全部通过。

### Notes
- `cmd/media-node/main.go`：新增媒体节点服务、周期扫描、心跳和优雅关闭。
- `internal/config/config.go`：新增节点总流量上限配置。
- `internal/media/scanner.go`：新增白名单扫描、ffprobe 元数据、FFmpeg 封面、增量缓存和媒体键映射。
- `internal/media/node.go`：新增节点鉴权、媒体/封面服务、总出口限速、心跳和清单同步。
- `internal/media/capacity_unix.go`：新增 Linux/FNOS 文件系统容量采集。
- `internal/media/capacity_windows.go`：提供 Windows 开发环境容量兼容实现。
- `internal/media/scanner_test.go`：验证目录规则、缓存、路径安全、Range、鉴权和真实媒体。
- `internal/store/catalog.go`：新增节点、工作室、视频目录、管理员编辑、播放会话和仪表盘存储。
- `internal/store/postgres.go`：新增后台用户查询和停用能力。
- `internal/store/types.go`：扩展媒体、节点、仪表盘和播放权限类型。
- `internal/store/postgres_integration_test.go`：扩展节点在线窗口和单播放会话测试。
- `internal/httpapi/router.go`：接入账号限速器和播放权限错误。
- `internal/httpapi/catalog.go`：新增游客目录、海报、已购库和后台内容/用户/节点接口。
- `internal/httpapi/media.go`：新增节点同步、心跳、播放会话和 Range 流代理。
- `internal/httpapi/media_integration_test.go`：新增网关到媒体节点的端到端代理测试。
- `go.mod`、`go.sum`：加入 Linux 文件系统容量依赖并更新校验值。
- `internal/httpapi/routes_pending.go`：删除临时路由占位，替换为正式目录与媒体路由。
- 回滚方式：回滚至上一条进度记录对应提交点，或删除本条列出的媒体、目录文件并恢复路由占位；数据库尚未部署生产环境，可重建独立测试库。

## 2026-07-31 - Task: 完成品牌前端、移动端与管理后台
### What was done
- 完成“小鹿叼着鹿币”的原创品牌标志、favicon 和应用图标，并实现游客目录、工作室、视频详情、播放、鹿币充值/兑换、会员、已购和账户页面。
- 完成响应式管理员概览、视频标题/工作室/封面/上下架、兑换码导出、订单、用户启停与密码重置、节点重扫界面。
- 移除存在高危间接依赖的测试工具链，改用 Vue 原生挂载测试，依赖审计归零。

### Testing
- `npm run build`：Vue TypeScript 检查和 Vite 生产构建通过。
- `npm run test`：2 个测试文件、3 个组件/格式测试全部通过。
- `npm run test:e2e`：桌面目录和锁定播放器、390px 手机双列目录与五项底栏、管理员概览共 3 项 Playwright 测试通过。
- `npm audit --audit-level=high`：0 个漏洞。
- Playwright 截图检查：页面无横向溢出，长标题换行，播放器弹窗、移动底栏和后台指标无重叠。

### Notes
- `web/package.json`：新增 Vue/Vite/Vitest/Playwright 任务和依赖。
- `web/package-lock.json`：锁定前端依赖，审计结果为零漏洞。
- `web/tsconfig.json`、`web/vite.config.ts`：配置严格 TypeScript、Vue 构建和测试边界。
- `web/index.html`、`web/public/manifest.webmanifest`：建立站点入口和安装清单。
- `web/public/favicon.svg`、`web/public/icon-192.png`、`web/public/icon-512.png`：新增简化品牌应用图标。
- `web/src/assets/deer-mark.svg`、`web/src/assets/deer-mark-mono.svg`：新增彩色与单色小鹿叼鹿币标志。
- `web/scripts/render-icons.mjs`：新增从矢量标志稳定生成 PNG 图标的脚本。
- `web/src/main.ts`、`web/src/env.d.ts`：新增 Vue 应用启动和类型声明。
- `web/src/api.ts`、`web/src/types.ts`、`web/src/format.ts`：新增请求、业务类型和显示格式工具。
- `web/src/App.vue`：实现完整用户端、播放器、鹿币交易与管理后台交互。
- `web/src/styles.css`：实现珊瑚红、炭黑、雾白和森林绿的桌面/手机响应式界面。
- `web/src/components/BrandLogo.vue`、`AppModal.vue`、`VideoCard.vue`：新增品牌、无障碍弹窗和稳定视频卡片组件。
- `web/src/App.test.ts`、`web/src/format.test.ts`：新增首屏与格式单元测试。
- `web/playwright.config.ts`、`web/e2e/app.spec.ts`：新增多视口视觉和交互验收。
- `web/e2e/assets/poster-1.svg`、`poster-2.svg`：新增不含真实敏感画面的中性视觉测试海报。
- `internal/store/catalog.go`、`internal/store/postgres.go`：新增后台工作室、封面记录和用户密码重置存储。
- `internal/httpapi/catalog.go`：新增后台工作室、JPEG 封面代理和密码重置接口。
- `internal/media/node.go`：新增鉴权 JPEG 封面写入。
- `internal/media/scanner_test.go`：新增封面写入与读取验证。
- 回滚方式：回滚本条列出的前端与后台扩展文件；品牌 PNG 可由 `npm run icons` 重新生成，测试海报与生产媒体无关联。

## 2026-07-31 - Task: 完成 Docker、飞牛部署文档与真实全栈冒烟
### What was done
- 提供本地、云端和飞牛 NAS 三套 Compose 落点，云端使用 Caddy 与 WireGuard，NAS 视频目录保持只读挂载，数据库、封面和 WireGuard 状态使用持久化存储。
- 补齐架构、飞牛部署、媒体规则、支付、安全、备份、接口和验证文档，并用两个真实 1080p MP4 完成注册、兑换、永久解锁和 Range 播放闭环。

### Testing
- `docker compose config`：本地配置有效；云端和飞牛 Compose 分别使用各自 `.env.example` 验证通过。
- `docker build --tag deerroom-app:dev .`、`docker build --tag deerroom-web:dev web`：后端/节点和前端镜像构建通过。
- 本地四服务健康；节点从只读目录识别 1 个工作室和 2 个 H.264/AAC 1080p MP4。
- 真实业务冒烟：新用户兑换 5 鹿币，永久解锁后余额为 4；播放流返回 `206`，读取 1024 字节，`Content-Range` 为 `bytes 0-1023/568611764`。

### Notes
- `.env.example`：提供本地与生产所需数据库、管理员、节点、限速和易支付变量样例。
- `Dockerfile`：构建 Gateway 与媒体节点并安装 FFmpeg 运行依赖。
- `compose.yaml`：提供可直接验证的本地 PostgreSQL、Gateway、媒体节点和 Web 栈。
- `web/Dockerfile`、`web/.dockerignore`、`web/Caddyfile`：构建并同源代理前端容器。
- `deploy/cloud/compose.yaml`、`deploy/cloud/.env.example`、`deploy/cloud/Caddyfile`、`deploy/cloud/wg0.conf.example`：提供云端生产部署落点。
- `deploy/media-node/compose.yaml`、`deploy/media-node/.env.example`、`deploy/media-node/wg0.conf.example`：提供飞牛 NAS 只读媒体节点部署落点。
- `scripts/dev-up.ps1`、`scripts/smoke.ps1`：提供本地启动和只读健康/目录冒烟命令。
- `README.md`：记录本地启动入口和生产文档位置。
- `docs/architecture.md`、`docs/deployment-fnos.md`、`docs/commerce-payment.md`、`docs/operations.md`、`docs/security.md`、`docs/testing.md`：记录正式架构、部署、支付、运维、安全和验证方法。
- `docs/api.md`、`docs/media-library.md`：记录接口和媒体编目规则。
- `progress.md`：追加本轮部署与真实冒烟证据。
- 回滚方式：在 `E:\AllProject\小鹿放映室` 执行 `docker compose down -v` 停止并删除本地测试栈；完整撤销可删除该独立项目目录，原网盘仓库不受影响。

## 2026-07-31 - Task: 完成大型媒体库分页与扫描稳定性收口
### What was done
- 视频公共目录和已购库改为每页 40 条、后台视频改为每页 100 条，前端以“加载更多”追加，支持几万条媒体时不再一次返回完整目录。
- 媒体扫描改为串行执行，30 秒心跳与首次扫描解耦，Linux/飞牛封面任务使用低调度优先级，完整清单同步超时放宽至 5 分钟。
- 有效会员在在线播放之外仍可用 1 鹿币永久解锁单片，并修复后台隐藏文件输入导致的横向溢出和操作按钮换行。

### Testing
- `$env:TEST_DATABASE_URL='postgres://postgres:test@localhost:55432/deerroom_test?sslmode=disable'; $env:TEST_MEDIA_ROOT='E:\BaiduNetdiskDownload'; go test ./...`：全部通过；真实 PostgreSQL 验证 45 条视频第一页 40、第二页 5、总数 45，搜索与工作室分页结果正确。
- `npm.cmd run build`、`npm.cmd run test`：构建通过，2 个测试文件、3 项单元测试通过。
- `npm.cmd run test:e2e`：4 项 Playwright 测试通过，覆盖目录追加、390px 手机双列与底栏、后台视频追加和会员永久解锁入口。
- `npm.cmd audit --audit-level=high`：0 个漏洞；桌面、手机和后台截图目视检查无横向溢出、重叠或按钮文字换行。
- 重建后的干净本地栈四服务健康，目录为 2 个视频和 1 个工作室；数据库仅保留 1 个引导管理员，无兑换批次、权益或支付订单。

### Notes
- `internal/media/scanner.go`：串行化扫描并降低 Linux FFmpeg 封面任务优先级。
- `internal/media/node.go`：独立运行节点心跳并延长大型清单同步超时。
- `internal/store/catalog.go`、`internal/store/types.go`：新增目录分页查询和统一分页响应。
- `internal/store/postgres_integration_test.go`：新增真实 PostgreSQL 的分页、搜索与工作室筛选验证。
- `internal/httpapi/catalog.go`：为公共目录、已购库和后台视频接入固定页大小。
- `web/src/types.ts`、`web/src/App.vue`：新增目录与后台加载更多状态，并保留会员永久解锁入口。
- `web/src/styles.css`：新增分页按钮布局并修复后台表格控件溢出和换行。
- `web/src/App.test.ts`、`web/e2e/app.spec.ts`：更新分页响应 mock 并扩展用户端、后台和会员流程验收。
- `docs/api.md`、`docs/media-library.md`：同步分页契约和大型扫描运行规则。
- `progress.md`：追加本轮规模适配与最终验收记录。
- 回滚方式：停止本地栈使用 `docker compose down`；代码完整回滚点为删除独立项目 `E:\AllProject\小鹿放映室`，或在首个 Git 基线提交后按本条文件清单逐项恢复。

## 2026-07-31 - Task: 规范化 Caddy 配置并清理启动警告
### What was done
- 使用 Caddy 官方格式化器统一本地和云端 Caddyfile 的换行与格式，不改变路由、反向代理或安全头行为。

### Testing
- `caddy validate --config`：本地和云端两份配置均返回 `Valid configuration`。
- Web 镜像重建并强制重建容器后状态为健康，启动日志不再出现 Caddyfile 格式警告。

### Notes
- `web/Caddyfile`：按 Caddy 官方格式规范化本地同源代理配置。
- `deploy/cloud/Caddyfile`：按 Caddy 官方格式规范化生产 HTTPS 代理配置。
- `progress.md`：追加本轮格式清理和验证证据。
- 回滚方式：重新运行 `caddy fmt --overwrite` 可得到相同结果；完整回滚可删除独立项目目录，路由行为无需数据回滚。

## 2026-07-31 - Task: 替换本地管理员账号凭据
### What was done
- 将现有本地管理员原位替换为用户指定邮箱，并通过应用现有 Argon2id 密码重置接口更新密码；未新增第二个管理员。
- 新增仅供本机 Compose 自动读取的忽略文件保存管理员覆盖项，真实凭据未写入示例配置、文档或 Git 跟踪文件。

### Testing
- Gateway 使用新环境变量重建后恢复健康，新账号登录成功且 `is_admin=true`。
- 旧邮箱分别使用原密码和新密码登录均返回 `401`；验证登录随后主动退出，数据库会话数为 0。
- 数据库保持 1 个启用管理员，媒体节点在 Gateway 重建后仍为健康状态。

### Notes
- `.env`：新增本地管理员邮箱和密码覆盖项，该文件已由 `.gitignore` 排除。
- `progress.md`：追加本轮管理员凭据变更和验证证据，未记录密码明文。
- 回滚方式：本地数据无需保留时可执行 `docker compose down -v`、删除 `.env` 后运行 `docker compose up -d --no-build`，恢复 Compose 默认引导管理员；需要保留数据时应先在后台重置密码，再更新管理员邮箱和 `.env`。

## 2026-07-31 - Task: 调整视频交互并改为纯兑换码鹿币体系
### What was done
- 将 NAS 目录设为视频编目的唯一来源，删除后台视频与工作室编辑入口、管理 API 和媒体节点封面写入接口，保留自动扫描、自动封面、节点状态与手动扫描。
- 将视频详情改为上方 16:9 封面、下方信息与操作的单列布局；解锁后保留播放按钮，并通过同标签 `/watch/{videoId}` 独立页面重新鉴权和创建播放会话。
- 将“我的”页面收敛为兑换码、鹿币流水和会员入口，彻底移除易支付运行能力、后台订单与现金换算描述；保留 `payment_orders` 历史表但应用不再读写。
- 后台用户管理新增邮箱模糊搜索、50 条分页加载，以及幂等的鹿币增加/扣减；调账备注可空、余额不得为负，钱包流水和管理员审计在同一事务内完成。
- 后台概览移除充值指标，改为鹿币发放与消费；同步更新接口、部署、媒体、安全和运维文档。

### Testing
- 真实 PostgreSQL 与真实媒体环境执行 `go test ./...`：全部通过，覆盖邮箱搜索分页、增加/扣减、空备注、负余额保护、重复 `request_id`、审计原子性，以及兑换、解锁、会员和媒体代理回归。
- `npm.cmd run build`、`npm.cmd run test`：前端生产构建通过，3 项单元测试通过。
- `npm.cmd run test:e2e`：6 项 Playwright 测试通过，覆盖详情单列布局、无详情会员入口、解锁后播放、`/watch/{id}` 深链接与后退、手机播放器、用户搜索和调账交互。
- `npm.cmd audit --audit-level=high`：0 个漏洞；手机目录和桌面账户页截图目视检查无重叠或横向溢出。
- 本地、云端和飞牛 NAS 三套 Compose 配置检查通过；后端与前端 Docker 镜像重建成功。
- 真实业务冒烟：管理员搜索命中、空备注调账与重复请求幂等通过，兑换后余额 7、解锁后余额 6；播放页返回 `200`，媒体 Range 返回 `206` 并读取 1024 字节，`Content-Range` 为 `bytes 0-1023/568611764`。
- 已删除的后台视频、后台订单和易支付路由均返回 `404`；源码与部署残留扫描仅命中保留表、404 回归测试和界面不存在易支付的断言。
- 使用干净卷重建本地栈后四服务均健康；自动扫描得到 1 个工作室、2 个视频，数据库仅有 1 个指定管理员，无兑换批次、兑换码、钱包流水、会员、权益、播放会话或审计记录。
- `gofmt -l .` 无输出；原网盘仓库仍为 `feature/distributed-netdisk-v1` 且无新增改动。

### Notes
- `.env.example`：删除易支付环境变量示例。
- `cmd/gateway/main.go`：移除易支付客户端初始化和网关注入。
- `compose.yaml`：删除本地易支付运行配置。
- `deploy/cloud/.env.example`：删除云端易支付环境变量示例。
- `deploy/cloud/compose.yaml`：删除云端易支付运行配置。
- `README.md`：将交易说明更新为兑换码、会员和管理员调账。
- `docs/api.md`：同步精简后的鹿币配置、用户分页调账、播放页和已删除接口。
- `docs/architecture.md`：同步 NAS 唯一编目来源与纯鹿币体系架构。
- `docs/commerce-payment.md`：删除旧易支付文档。
- `docs/credits.md`：新增兑换码、会员、钱包流水和管理员调账说明。
- `docs/media-library.md`：记录自动发布、不可人工编辑和独立播放页规则。
- `docs/operations.md`：更新部署与运维流程，移除支付配置。
- `docs/security.md`：更新调账幂等、审计和媒体访问边界。
- `internal/config/config.go`：删除易支付配置字段与读取逻辑。
- `internal/httpapi/catalog.go`：删除后台视频、工作室和封面管理路由，接入用户搜索分页与调账接口。
- `internal/httpapi/commerce.go`：精简鹿币配置响应并移除支付下单、通知和返回处理。
- `internal/httpapi/media_integration_test.go`：新增已删除路由 `404` 与播放边界回归验证。
- `internal/httpapi/router.go`：移除易支付、后台订单和后台视频管理路由注册。
- `internal/media/node.go`：删除媒体节点封面写入接口，保留自动封面读取。
- `internal/media/scanner_test.go`：删除人工封面写入测试并保留自动扫描验证。
- `internal/store/catalog.go`：删除视频人工编目存储方法，新增管理员用户搜索分页和事务调账。
- `internal/store/commerce.go`：删除支付订单读写逻辑，保留兑换、解锁、会员与钱包能力。
- `internal/store/postgres.go`：移除支付运行初始化并保留历史表迁移兼容。
- `internal/store/postgres_integration_test.go`：新增搜索、调账、幂等、余额保护和审计集成测试。
- `internal/store/types.go`：删除支付类型，新增用户分页和调账响应类型。
- `internal/zpay/zpay.go`：删除易支付签名实现。
- `internal/zpay/client.go`：删除易支付收银台和查单客户端。
- `internal/zpay/zpay_test.go`：删除易支付包测试。
- `web/src/App.vue`：重构详情、独立播放页、账户页、后台导航和用户调账流程。
- `web/src/styles.css`：实现详情单列、独立播放器和调账弹窗的桌面与手机布局。
- `web/src/types.ts`：删除支付与后台视频类型，新增用户分页和调账类型。
- `web/src/App.test.ts`：更新精简鹿币配置和首屏回归 mock。
- `web/e2e/app.spec.ts`：扩展详情、播放深链接、手机播放器和管理员调账验收。
- `progress.md`：仅在末尾追加本轮施工、验证和回滚记录。
- 回滚方式：先在 `E:\AllProject\小鹿放映室` 执行 `docker compose down -v` 停止并清理本地运行数据；当前仓库尚无提交基线，需完整撤销时可删除该独立项目目录，原网盘仓库不会受影响。

## 2026-07-31 - Task: 合并工作室入口并升级随机首页与开源播放器
### What was done
- 删除独立工作室视图及桌面、手机导航入口，将原有工作室筛选条固定在首页；首页默认随机放映，搜索和工作室筛选继续保持原有排序。
- 为公开视频目录增加正整数随机种子，使用 PostgreSQL 稳定哈希支持随机分页；每次重新进入首页更换顺序，同一次加载更多沿用种子并按视频编号去重。
- 使用 MIT Artplayer 5.4.0 替换浏览器原生播放器，启用倍速、画中画、网页/系统全屏、键盘和移动端操作，并使用珊瑚红与暖灰样式统一“小鹿放映室”品牌。
- 关闭 Artplayer 右键菜单和版本输出，移除其隐藏的官网链接节点；未接入广告、下载、截图、外部脚本或第三方统计。
- 将播放页改为网站统一的暖雾白背景，保留黑色视频画布，并完善桌面和手机端标题、工作室与媒体信息布局。

### Testing
- 使用独立 PostgreSQL 17 数据库运行 `go test ./...`：全部通过；验证相同种子顺序一致、不同种子顺序变化、45 部视频跨页无重复，以及搜索和工作室筛选回归。
- `npm.cmd run build`：Vue TypeScript 检查和 Vite 生产构建通过；Artplayer 随前端产物本地打包。
- `npm.cmd run test`：2 个测试文件、3 项单元测试通过；首页首次目录请求包含随机种子。
- `npm.cmd run test:e2e`：8 项 Playwright 测试通过，覆盖随机首页、种子刷新、首页工作室筛选、详情、独立播放页、返回销毁、深链接、桌面/手机布局和账户后台回归。
- 播放器右键验收确认不存在 `ArtPlayer` 品牌文本或任何 HTTP 外链；依赖审计结果为 0 个漏洞。
- Playwright 桌面和手机截图目视检查通过：暖雾白背景、16:9 播放区域、控件、长标题和移动底栏均无重叠或横向溢出。
- 本地、云端和飞牛 NAS 三套 Compose 配置检查通过；由于本机 BuildKit gRPC 会话头异常，改用 `DOCKER_BUILDKIT=0 docker build` 成功构建 `deerroom-app:dev` 和 `deerroom-web:dev`。
- 新镜像替换后四服务均健康；实时目录相同种子顺序稳定，非法种子返回 `422`。
- 真实 NAS 视频 Range 冒烟返回 `206`，读取 1024 字节，`Content-Range` 为 `bytes 0-1023/467899649`；临时权益和全部测试会话已清理。
- 最终数据库保持 1 个管理员、1 个工作室和 2 个视频，会员、登录会话与播放会话均为 0；`gofmt -l .` 无输出，原网盘仓库状态未改变。

### Notes
- `README.md`：补充随机首页和无广告、无外链自托管播放器说明。
- `docs/api.md`：记录随机种子接口、兼容规则和 Artplayer 播放边界。
- `docs/media-library.md`：记录首页随机目录与工作室筛选的合并行为。
- `docs/testing.md`：补充随机分页和播放器外链验收说明。
- `internal/httpapi/catalog.go`：解析并校验公开目录的正整数随机种子。
- `internal/httpapi/auth_test.go`：新增有效和非法随机种子测试。
- `internal/store/catalog.go`：为视频分页增加稳定哈希随机排序。
- `internal/store/postgres_integration_test.go`：新增随机顺序稳定性、差异性和跨页唯一性验证。
- `web/package.json`：锁定 Artplayer 5.4.0 运行依赖。
- `web/package-lock.json`：锁定 Artplayer 及其间接依赖版本和校验值。
- `web/src/components/VideoPlayer.vue`：新增无广告、无外链的 Artplayer 生命周期封装与播放配置。
- `web/src/App.vue`：合并首页和工作室入口、管理随机种子，并接入独立播放器组件。
- `web/src/styles.css`：统一播放页品牌背景并定制播放器桌面、手机样式。
- `web/src/App.test.ts`：验证首页标题、筛选条、独立导航移除和随机种子请求。
- `web/e2e/app.spec.ts`：扩展随机首页、工作室筛选、Artplayer 外链与多视口验收。
- `progress.md`：仅在末尾追加本轮施工、验证与回滚记录。
- 回滚方式：在 `E:\AllProject\小鹿放映室` 执行 `docker compose down` 停止当前服务；当前仓库尚无提交基线，需完整撤销时可删除该独立项目目录，原网盘仓库不会受影响。

## 2026-07-31 - Task: 调整兑换码 TXT 导出、播放页信息与全站滚动条
### What was done
- 兑换码批次创建改为下载 UTF-8 TXT 文件，正文仅包含兑换码且每行一个；前端下载扩展名和管理员按钮文案同步改为 TXT。
- 独立播放页保留顶部及播放器下方的工作室、视频名称，移除分辨率、视频和音频编码技术信息。
- 全站隐藏滚动条视觉元素，同时保留页面、弹窗、横向筛选条和后台表格的滚动能力；同步更新兑换码接口与业务说明文档。

### Testing
- `gofmt -w internal/httpapi/commerce.go internal/httpapi/commerce_test.go`：通过。
- `go test ./...`：全部 Go 测试通过，包含新增兑换码“每行一个纯码”契约测试。
- `npm.cmd run build`：前端 TypeScript 检查与生产构建通过。
- `npm.cmd run test`：2 个测试文件、3 项单元测试通过。
- `npm.cmd run test:e2e`：8 项 Playwright 测试通过，新增播放页技术信息隐藏和 TXT 导出按钮断言；桌面/移动端无横向溢出。
- `git diff --check`：通过。

### Notes
- `internal/httpapi/commerce.go`：将兑换码批次导出响应改为纯文本 TXT。
- `internal/httpapi/commerce_test.go`：新增每行一个兑换码且不含 CSV 字段的单测。
- `web/src/App.vue`：同步 TXT 下载与按钮文案，删除独立播放页技术元数据。
- `web/src/styles.css`：跨浏览器隐藏滚动条但保留滚动行为。
- `web/e2e/app.spec.ts`：增加播放页元数据隐藏和 TXT 操作入口验收。
- `docs/credits.md`、`docs/api.md`：同步 TXT 导出接口和业务说明。
- 回滚方式：回滚本条列出的文件即可恢复 CSV 导出、播放页技术信息和原滚动条样式；运行中的本地服务可用 `docker compose down` 停止。

## 2026-07-31 - Task: 统一目录工作室标题字体
### What was done
- 将目录标题改为站点默认无衬线字体，使“半岛2024”等中英文与数字组合使用一致的正常字形；其他页面标题字体保持不变。

### Testing
- `npm.cmd run build`：前端 TypeScript 检查与生产构建通过。
- `npm.cmd run test`：2 个测试文件、3 项单元测试通过。
- `npm.cmd run test:e2e`：8 项 Playwright 测试通过，工作室筛选、桌面和移动端布局均无回归。

### Notes
- `web/src/styles.css`：仅覆盖目录标题字体族为站点默认字体。
- `progress.md`：追加本轮修改与验证记录。
- 回滚方式：删除 `web/src/styles.css` 中 `.catalog-heading h1 { font-family: inherit; }` 规则即可恢复原展示字体。

## 2026-07-31 - Task: 增加登录注册图片验证码与轻量防刷
### What was done
- 新增本站自托管 5 位 PNG 图片验证码，挑战绑定客户端与用途、5 分钟有效、最多尝试 5 次并一次性消费；注册始终校验，登录首次失败后由服务端强制后续验证码。
- 增加注册、登录和验证码签发的内存频率窗口，以及默认 4 个 Argon2 计算任务的并发闸门；所有状态自动过期且不写数据库。
- 前端注册弹窗自动加载验证码，登录失败后切换到验证码模式，并支持图片刷新和验证码错误后的自动换图；同步更新接口、安全和验证说明。

### Testing
- `go test ./...`：全部通过，覆盖 PNG 尺寸、客户端/用途绑定、过期、一次性消费、五次失败失效和注册强制验证码。
- `npm.cmd run build`：Vue TypeScript 检查与生产构建通过。
- `npm.cmd run test`：2 个测试文件、4 项单元测试通过，包含注册模式验证码交互。
- `npm.cmd run test:e2e`：8 项 Playwright 回归测试通过。

### Notes
- `internal/httpapi/captcha.go`、`captcha_test.go`：新增验证码、认证频率保护和契约测试。
- `internal/httpapi/auth.go`、`router.go`：接入注册/登录验证码、Argon2 并发闸门和验证码接口。
- `internal/config/config.go`、`cmd/gateway/main.go`、`.env.example`、`compose.yaml`、`deploy/cloud/.env.example`、`deploy/cloud/compose.yaml`：接入密码计算并发配置。
- `go.mod`、`go.sum`：加入并锁定 PNG 字体渲染依赖。
- `web/src/api.ts`、`web/src/App.vue`、`web/src/styles.css`、`web/src/App.test.ts`：实现验证码错误识别、表单交互和响应式样式。
- `docs/api.md`、`docs/security.md`、`docs/testing.md`：记录验证码契约和验证入口。
- `progress.md`：追加本阶段施工、验证和回滚记录。
- 回滚方式：回滚本条列出的文件并重新构建 Gateway/Web 镜像；未产生数据库迁移或用户数据变更。

## 2026-07-31 - Task: 增加单账号播放与节点连接保护
### What was done
- 保留单账号共享带宽限速，并增加默认 4 个并发媒体 GET、每分钟 120 个流请求的内存保护；超限临时返回 `429`，不记录流量、不冻结账号。
- 媒体节点增加默认 64 个同时媒体连接上限，超限返回 `503`，节点总出口限速保持不变。
- 创建播放会话时锁定用户记录，修复并发请求可能产生多个有效播放会话的竞态；同步补充配置项与安全说明。

### Testing
- `go test ./...`：全部通过，覆盖账号并发连接、请求窗口重置和节点繁忙响应。
- `go test -race ./...`：全部通过，无竞态报告。
- 新增真实 PostgreSQL 并发播放会话集成用例；默认测试未配置 `TEST_DATABASE_URL` 时按既有规则跳过，最终全量验收将使用独立测试库执行。

### Notes
- `internal/httpapi/stream_guard.go`、`stream_guard_test.go`、`media.go`、`router.go`：新增账号流请求保护并接入播放代理。
- `internal/media/node.go`、`scanner_test.go`：新增节点媒体连接上限和验证。
- `internal/store/catalog.go`、`postgres_integration_test.go`：串行化账号播放会话并补并发集成测试。
- `internal/config/config.go`、`cmd/gateway/main.go`、`cmd/media-node/main.go`、各级 `.env.example` 与 Compose：接入播放和节点连接配置。
- `docs/security.md`、`docs/architecture.md`：同步限速和并发边界。
- `progress.md`：追加本阶段施工、验证和回滚记录。
- 回滚方式：回滚本条列出的文件并重建 Gateway/媒体节点镜像；未修改数据库结构或现有业务数据。

## 2026-07-31 - Task: 完善多 Docker 节点与目录同步
### What was done
- 将生产 Gateway 节点鉴权改为按名称绑定独立 API Token、Relay Token 和固定内部地址，保留本地单节点旧配置兼容；媒体代理按视频所属节点选择读取凭据。
- 节点 Compose 支持从 `.env` 生成唯一 WireGuard 配置，节点名、项目名、Gateway/节点地址、本地目录和限速均可复制配置；补充第二节点样例与云端多 Peer 示例。
- 扫描器新增基于媒体键、大小和修改时间的目录版本，未变化时只发心跳；变化时按工作室去重并批量 Upsert 视频，手动扫描异步返回并通过内存状态显示扫描中/成功/失败。
- 公开目录和工作室只返回 90 秒内在线节点的内容；单节点离线不影响其他节点，节点恢复后自动重新出现。

### Testing
- 使用临时 PostgreSQL 17 独立测试库运行 `go test -race ./...`：全部通过，覆盖批量同步、并发播放会话和双节点离线隔离；临时容器随后删除。
- `go test ./...`：全部通过，覆盖节点凭据名称/地址绑定、目录版本稳定、未变化只同步一次和扫描状态。
- 本地、云端、节点一、节点二四套 Compose 配置检查均通过。
- `npm.cmd run build`、`npm.cmd run test`：前端构建通过，2 个测试文件、4 项单元测试通过。
- `npm.cmd run test:e2e`：10 项 Playwright 测试通过，新增登录失败验证码和后台节点扫描状态验收。

### Notes
- `internal/config/config.go`、`cmd/gateway/main.go`、`internal/httpapi/router.go`、`media.go`、`catalog.go`、`node_state.go`：接入按节点凭据和内存扫描状态。
- `internal/media/scanner.go`、`node.go`：新增目录版本、跳过未变化同步和异步扫描。
- `internal/store/catalog.go`、`types.go`：批量写入目录、返回节点内部名称并隐藏离线目录。
- `internal/config/config_test.go`、`internal/httpapi/node_credentials_test.go`、`internal/media/scanner_test.go`、`internal/store/postgres_integration_test.go`：新增配置、身份、版本和双节点验证。
- `deploy/media-node/compose.yaml`、`.env.example`、`.env.node-2.example`、`wg0.conf.example`：提供可复制节点和自动 WireGuard 配置。
- `deploy/cloud/compose.yaml`、`.env.example`、`wg0.conf.example`：提供多节点凭据与 Peer 配置。
- `web/src/App.vue`、`types.ts`、`e2e/app.spec.ts`：显示节点扫描状态并扩展端到端验收。
- `README.md`、`docs/api.md`、`docs/architecture.md`、`docs/deployment-fnos.md`、`docs/media-library.md`、`docs/security.md`：同步多节点与扫描运行规则。
- `progress.md`：追加本阶段施工、验证和回滚记录。
- 回滚方式：恢复本条列出的配置、节点和目录代码后重建 Gateway/媒体节点/Web 镜像；未产生数据库迁移，现有节点和视频数据无需回滚。

## 2026-07-31 - Task: 完成运行时收口与全量验收
### What was done
- Go 构建版本升级到 1.25.12；Gateway 和节点增加建连、响应头、读取和空闲连接边界，应用启动与每小时维护任务清理过期登录/播放会话。
- 本地、云端和媒体节点 Compose 增加只读根文件系统、临时目录、进程数/内存上限和 `no-new-privileges`；WireGuard 镜像固定到不可变 digest；根 Compose 的节点项目名、地址和目录也完成参数化。
- 修复验证码依赖的已知漏洞，将 `golang.org/x/image` 升级到 0.39.0；同步更新运维和节点部署文档。

### Testing
- `gofmt -l .`：无输出。
- `go test -race ./...`：全部通过；此前使用临时 PostgreSQL 17 独立库再次通过真实批量同步、离线隐藏、并发播放和会话清理用例，临时容器已删除。
- `go vet ./...`：通过。
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`：0 个代码可达漏洞。
- `npm.cmd audit --audit-level=high`：0 个漏洞；`npm.cmd run build`、`npm.cmd run test`：构建通过，4 项单元测试通过；`npm.cmd run test:e2e`：10 项 Playwright 测试通过。
- 本地、云端、节点一、节点二 `docker compose config -q`：全部通过；Web 镜像 `deerroom-web:dev` 构建成功。
- Gateway 应用镜像已使用本地缓存的 `golang:1.25.12-alpine` 基础镜像及 `https://goproxy.cn,direct` 成功重建；Gateway、媒体节点、Web 和 PostgreSQL 已启动并通过健康检查，公开视频目录接口返回正常。

### Notes
- `go.mod`、`go.sum`、`Dockerfile`：升级 Go 和验证码渲染依赖，固定应用构建基线。
- `cmd/gateway/main.go`、`cmd/media-node/main.go`、`internal/httpapi/router.go`、`internal/media/node.go`：增加 HTTP 边界、客户端传输配置和维护启动逻辑。
- `internal/store/maintenance.go`、`postgres_integration_test.go`：新增过期会话清理与真实数据库验证。
- `compose.yaml`、`deploy/cloud/compose.yaml`、`deploy/media-node/compose.yaml`：增加资源限制、只读运行和不可变 WireGuard 镜像。
- `docs/operations.md`、`docs/deployment-fnos.md`、`README.md`：同步运行维护和多节点部署说明。
- `progress.md`：追加最终施工、验证和外部构建缺口。
- 回滚方式：回滚本条列出的运行时、依赖、Compose 和文档文件并重建镜像；未执行数据库迁移，现有业务数据不需要回滚。需要回滚镜像时执行 `docker compose down` 后重新构建并启动对应服务。

## 2026-07-31 - Task: 统一中文与英文界面字体
### What was done
- 移除界面中的特殊展示字体和等宽字体指定，标题、数字、视频名称与中文正文统一使用系统常规无衬线字体栈；字号、布局和播放器行为保持不变。

### Testing
- `npm.cmd run build`：前端类型检查和生产构建通过。
- `npm.cmd run test`：2 个测试文件、4 项单元测试通过。
- `npm.cmd run test:e2e`：10 项 Playwright 验收全部通过。
- `docker compose build web`：Web 镜像按最新字体构建成功；重启后 HTTP 返回的 CSS 含系统字体栈且不含 `Newsreader`，Gateway 健康接口仍返回 `ok`。

### Notes
- `web/src/styles.css`：统一全局、标题、账余额度和视频时长标签的字体设置。
- `progress.md`：追加本次字体修正、验证和回滚记录。
- 回滚方式：恢复 `web/src/styles.css` 本轮变更并重新执行 `npm.cmd run build`；不涉及数据库、接口或媒体文件。

## 2026-07-31 - Task: 清理过期认证限流状态
### What was done
- 为注册、登录、验证码签发和登录验证码状态增加低频惰性清理；状态仍只保存在 Gateway 内存，限流窗口和用户流程不变，避免大量一次性客户端地址造成无界键增长。

### Testing
- `gofmt -l .`：无未格式化 Go 文件。
- `go test ./...`：全部通过，包含过期认证状态清理测试。
- `go test -race ./...`：全部通过，无竞态报告。
- `docker build --build-arg GOPROXY=https://goproxy.cn,direct --tag deerroom-app:dev .`：应用镜像重建成功；Gateway 和媒体节点重启后均为 healthy，健康接口返回 `ok`，视频目录接口返回 2 条记录。

### Notes
- `internal/httpapi/captcha.go`：清理过期认证窗口和验证码要求状态。
- `internal/httpapi/captcha_test.go`：覆盖过期限流状态回收。
- `progress.md`：追加本轮内存状态修正、验证和回滚记录。
- 回滚方式：恢复上述两个 Go 文件本轮变更并重新构建 Gateway；不涉及数据库结构或用户数据。

## 2026-07-31 - Task: 同步认证状态生命周期文档
### What was done
- 安全文档补充认证验证码、限流窗口和登录失败状态会在 Gateway 内存中自动清理过期项的说明。

### Testing
- 文档内容与已通过的认证状态清理单元测试和 `go test -race ./...` 结果一致；未引入代码变更。

### Notes
- `docs/security.md`：补充内存认证状态生命周期说明。
- `progress.md`：追加本次文档同步记录。
- 回滚方式：恢复 `docs/security.md` 本轮一行文档变更；不涉及代码、数据库或运行时配置。

## 2026-07-31 - Task: 移除会员并增加兑换说明公告
### What was done
- 彻底移除会员购买、会员期限、会员统计和会员权限判断；开发数据库按用户确认重置，初始化结构保留管理员、普通成员、鹿币、兑换码和永久视频权益，不再创建会员表。
- 所有视频永久解锁统一为 1 鹿币；普通成员账户页保留余额、兑换码和已购视频入口，后台用户页只保留成员状态、鹿币调整和密码管理。
- 新增兑换说明站点设置：管理员可在兑换码后台编辑最多 2000 字的纯文本说明，成员充值区自动展示；更新操作写入管理员审计。

### Testing
- `go test ./...`、`go test -race ./...`：通过；独立 PostgreSQL 17 集成库通过新结构、兑换说明持久化、兑换码并发和播放会话测试。
- `go vet ./...`：通过。
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`：无代码可达漏洞。
- `npm.cmd run build`、`npm.cmd run test`：构建通过，4 项单元测试通过。
- `npm.cmd run test:e2e`：10 项 Playwright 测试通过，覆盖成员充值说明和管理员编辑。
- `npm.cmd audit --audit-level=high`：0 个漏洞。
- 开发 Compose 数据库重置后，PostgreSQL、Gateway、媒体节点和 Web 均 healthy；`/api/v1/health` 返回 `ok`，`/api/v1/commerce` 返回 `video_price: 1`，数据库确认无 `memberships` 表、有 `site_settings` 表。

### Notes
- `internal/store/migrations/001_initial.sql`、`internal/store/types.go`、`internal/store/postgres.go`、`internal/store/catalog.go`、`internal/store/commerce.go`：移除会员结构和逻辑，加入站点设置与统一视频解锁规则。
- `internal/store/settings.go`、`internal/httpapi/commerce.go`、`internal/httpapi/router.go`：新增兑换说明读写接口和审计。
- `web/src/App.vue`、`web/src/types.ts`、`web/src/styles.css`、`web/src/App.test.ts`、`web/e2e/app.spec.ts`：移除会员界面并增加充值说明编辑/展示。
- `docs/api.md`、`docs/credits.md`、`internal/store/postgres_integration_test.go`、`internal/httpapi/media_integration_test.go`：同步接口文档、业务说明和精简数据库测试。
- `progress.md`：追加本轮实施、验证和回滚记录。
- 回滚方式：源码可恢复本轮列出的文件后重新构建镜像；本次开发 PostgreSQL 和封面卷已按确认执行 `docker compose down -v`，原有开发数据已不可恢复，不能通过代码回滚找回。

## 2026-07-31 - Task: 增加本地 7z 一键解压工具
### What was done
- 增加 Windows 一键启动器和 PowerShell 脚本，默认处理 `E:\BaiduNetdiskDownload\悠米` 下的全部 `.7z` 文件。
- 脚本自动从 `说明.txt` 读取解压密码；每个压缩包先完整测试并解压到临时目录，确认没有目标文件冲突后才移动文件并删除原压缩包，失败时保留原压缩包。
- 增加 `-WhatIf` 预览模式和本地使用说明，不把密码写入脚本或文档。

### Testing
- PowerShell AST 解析：通过。
- `extract-7z-and-delete.ps1 -WhatIf`：通过，检测到当前 16 个 `.7z` 文件，未执行解压或删除。
- 实际解压未执行：当前 Windows 未安装 7-Zip，需安装后再运行启动器。

### Notes
- `scripts/extract-7z-and-delete.ps1`：安全解压、冲突检查和成功后删除逻辑。
- `scripts/extract-7z-and-delete.cmd`：双击启动器。
- `docs/local-archive-extraction.md`：安装要求、默认目录和使用方式。
- `progress.md`：追加本轮变更、验证和回滚记录。
- 回滚方式：删除上述三个新增脚本/文档文件；不会影响 Docker、数据库或现有视频文件。若脚本已执行造成文件移动或压缩包删除，只能从已有文件或备份恢复，代码回滚无法恢复被删除的压缩包。

## 2026-07-31 - Task: 执行本地 7z 批量解压
### What was done
- 在确认 7-Zip 已安装后，使用一键脚本处理了当时目录中已完成下载的 19 个 `.7z` 文件；所有压缩包均通过密码和完整性测试，成功解压后删除。
- 处理过程中发现有新压缩包陆续下载，脚本随后又处理了新出现的文件；未对下载中的文件执行解压。

### Testing
- 7-Zip 26.02 x64：可用。
- 每个实际处理的压缩包均返回 `Everything is Ok`，脚本返回成功。
- 最终检查：`E:\BaiduNetdiskDownload\悠米` 下无 `.7z` 文件、无 `.extract-*` 临时目录，共 20 个 MP4，约 27.32 GiB。

### Notes
- 本轮只改变了本地媒体目录中的压缩包和解压文件，不改变项目代码或数据库。
- 后续新下载的 `.7z` 不会自动触发处理，需要再次运行 `scripts\extract-7z-and-delete.cmd`。
- 回滚方式：已删除的压缩包无法通过代码回滚恢复；如需保留原始压缩包，应在以后运行前复制到其他位置。

### Count correction
- 本轮累计成功处理 20 个 `.7z` 压缩包；上一段 `What was done` 中的 19 个为阶段性计数，最终验收以 20 个 MP4、无残留 `.7z` 为准。

## 2026-07-31 - Task: 更新 Web 容器并诊断节点新增视频不显示
### What was done
- 重建并重启本地 Web 容器，部署包含最新后台、TXT 兑换码和播放页改动的前端镜像。
- 管理后台节点重新扫描改为轮询扫描状态，扫描完成后自动刷新工作室和视频目录，扫描失败时显示节点错误。
- 定位节点扫描失败原因：Windows bind mount 下有 7 个 MP4 文件名超过 Linux 255 字节文件名限制，导致 `/media/悠米` 返回 `input/output error`，数据库不会收到新增清单。
- 未自动重命名视频，避免未经确认改变视频标题；部署文档补充了文件名限制和处理方式。

### Testing
- `docker compose build web`：通过，Web 镜像构建成功。
- Web、Gateway、媒体节点、PostgreSQL：健康检查通过。
- `npm.cmd run test`：2 个测试文件、4 项测试通过。
- `git diff --check`：通过。
- 实际节点状态：宿主机有 20 个 MP4，但节点当前只能读取其中可兼容的部分；长文件名修正后需点击“重新扫描”。

### Notes
- `web/src/App.vue`：增加节点扫描完成轮询和目录自动刷新。
- `docs/deployment-fnos.md`：补充 Windows Docker 文件名兼容限制。
- `progress.md`：追加本轮部署、诊断和验证记录。
- 回滚方式：恢复 `web/src/App.vue` 和 `docs/deployment-fnos.md` 本轮改动后重新构建 Web；不涉及数据库和视频文件。视频文件未被本轮代码自动重命名。

## 2026-07-31 - Task: 自动兼容 Windows 超长视频文件名
### What was done
- 增加 Windows 媒体兼容视图：源目录中的视频保持原名，视图目录使用同盘硬链接；超过文件名限制的文件自动使用短哈希别名。
- 生成 `catalog-titles.json` 保存原始相对路径和完整标题，媒体节点扫描别名后恢复网页中的完整标题；兼容 UTF-8 BOM。
- 增加 20 秒轮询监听脚本，新增或变化的视频会自动更新兼容视图；开发启动脚本会先生成一次视图。
- 本地 Docker 媒体挂载已切换到 `E:\BaiduNetdiskDownload\.deer-media-view`，节点实际读取 22 个视频，7 个超长文件名均显示完整原始标题。

### Testing
- `go test ./internal/media`、`go test ./...`：通过。
- PowerShell 脚本 AST 解析：通过。
- `npm.cmd run build`、`npm.cmd run test`：构建通过，4 项测试通过。
- `docker build --tag deerroom-app:dev .`：通过；Gateway 和媒体节点健康。
- `/api/v1/videos`：返回 22 条；别名标题数量 0，长标题恢复数量 7。
- 原始媒体目录未重命名、未复制视频内容、未删除原文件。

### Notes
- `internal/media/scanner.go`、`internal/media/scanner_test.go`：读取标题清单并覆盖缓存命中场景。
- `scripts/prepare-media-view.ps1`：生成同盘硬链接和标题清单。
- `scripts/watch-media-view.ps1`、`scripts/watch-media-view.cmd`：持续更新兼容视图。
- `scripts/dev-up.ps1`：开发启动时先生成兼容视图。
- `.env`：本地媒体节点改挂兼容视图目录。
- `docs/deployment-fnos.md`、`docs/local-archive-extraction.md`：补充使用方式。
- `progress.md`：追加本轮兼容层实施和验证记录。
- 回滚方式：将 `.env` 的 `DEER_MEDIA_HOST_PATH` 改回原始目录并重建节点；删除兼容视图目录不会影响原始视频，但正在运行的监听器需手动关闭。

## 2026-07-31 - Task: 优化猜你喜欢分页与网页拖拽行为
### What was done
- 首页和工作室目录统一按“猜你喜欢”入口展示；每次进入首页生成新的随机种子，视频目录接口和已购库单页上限统一为 50 条。
- 工作室/目录页面增加右下角上一页、页码、下一页控件，切页后自动回到页面顶部；移除目录“加载更多”逻辑。
- 禁止页面、图片、链接和视频的原生拖拽与文件拖放，播放器控件仍可正常操作。
- 更新 API 文档、前端单元测试和 Playwright mock 分页数据。

### Testing
- `go test ./...`：通过。
- `npm.cmd run build`：通过。
- `npm.cmd run test`：2 个测试文件、4 项测试通过。
- `git diff --check`：通过。
- 项目整理审计：未发现可安全删除的业务源码、迁移、配置或媒体运行文件；`web/dist`、`web/test-results` 为可再生成产物，但当前安全策略拒绝递归删除命令，因此未删除。

### Notes
- `internal/httpapi/catalog.go`：公开目录和已购库分页大小改为 50。
- `internal/store/postgres_integration_test.go`：同步分页集成断言。
- `web/src/App.vue`：猜你喜欢文案、分页控件和拖拽禁用。
- `web/src/styles.css`：禁用图片、链接、视频原生拖拽并增加分页样式。
- `web/src/App.test.ts`、`web/e2e/app.spec.ts`：同步测试数据与交互断言。
- `docs/api.md`、`progress.md`：同步接口说明和施工记录。
- 回滚方式：恢复上述源码、测试和文档文件后重新构建 Web/Gateway；不涉及数据库、原始视频和媒体兼容视图。

## 2026-07-31 - Task: 完成分页优化、项目整理和新增视频解压
### What was done
- 重建并重启 Web/Gateway/媒体节点，首页实际使用 50 条分页，节点目录已包含新增视频。
- 解压并删除新增的 20 个 `.7z` 压缩包，媒体目录 MP4 总数由 20 增加到 40；兼容视图和数据库目录同步为 42 条（含原有半岛2024视频）。
- 保持原始视频文件名和完整介绍，兼容视图继续使用同盘硬链接和标题清单。
- 完成源码引用审计；没有删除业务源码、数据库迁移、媒体节点脚本或运行配置。`web/dist`、`web/test-results` 属于可再生成产物，但当前终端安全策略拒绝递归删除，因此保留。

### Testing
- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm.cmd run build`：通过。
- `npm.cmd run test`：4 项通过。
- `npm.cmd run test:e2e`：10 项通过。
- `/api/v1/videos?page=1&seed=123`：实际返回 `page_size=50`、总数 42。
- Docker Gateway、媒体节点、Web、PostgreSQL：健康检查通过。
- `git diff --check`：通过。

### Notes
- `internal/httpapi/catalog.go`、`internal/store/postgres_integration_test.go`：目录分页上限统一为 50。
- `web/src/App.vue`、`web/src/styles.css`：猜你喜欢、分页、拖拽禁用。
- `web/src/App.test.ts`、`web/e2e/app.spec.ts`：同步分页和首页测试。
- `docs/api.md`：同步分页与猜你喜欢说明。
- `progress.md`：追加本轮收尾、解压和验证记录。
- 回滚方式：恢复本轮源码、测试和文档文件后重新构建镜像；已删除的 20 个原始 `.7z` 压缩包无法通过代码回滚恢复，解压后的 MP4 和兼容视图不受源码回滚影响。

## 2026-07-31 - Task: 执行半岛2024新增压缩包解压
### What was done
- 使用现有一键解压程序处理 `E:\BaiduNetdiskDownload\半岛2024` 下的 22 个 `.7z` 文件。
- 22 个压缩包均通过密码和完整性验证，成功解压后删除。
- 重新生成 Windows Docker 兼容视图并重建媒体节点目录，新增视频已同步到 Gateway。

### Testing
- 压缩包处理结果：22 个成功，0 个残留 `.7z`，0 个临时解压目录。
- 宿主机原始媒体目录 MP4：63 个。
- Docker 兼容视图 MP4：63 个。
- `/api/v1/videos`：返回 63 条视频。
- 媒体节点健康检查：通过。

### Notes
- 本轮只修改了本地媒体文件和 `progress.md`；原始 MP4 文件名保持不变，兼容视图继续使用硬链接和完整标题清单。
- 回滚方式：已删除的 22 个 `.7z` 原包无法通过代码恢复；解压后的 MP4 和兼容视图保留，不涉及数据库结构回滚。

## 2026-07-31 - Task: 优化 7z 直接解压路径
### What was done
- 将一键解压程序从临时目录解压后移动改为直接解压到压缩包所在目录，避免临时写入和额外移动。
- 保留密码和完整性校验；只有 7-Zip 成功返回后才删除压缩包，同名文件按用户要求覆盖。
- 兼容视图继续使用同盘硬链接，不产生视频副本。

### Testing
- PowerShell 脚本 AST 解析：待执行下一批压缩包时验证实际路径；当前没有待处理压缩包，未进行破坏性实压测试。
- `git diff --check`：通过。

### Notes
- `scripts/extract-7z-and-delete.ps1`：直接解压并成功后删除。
- `docs/local-archive-extraction.md`：同步覆盖和失败行为说明。
- `progress.md`：追加本轮记录。
- 回滚方式：恢复上述脚本和文档即可；已删除压缩包无法由代码恢复，已有 MP4 不受影响。

## 2026-07-31 - Task: 恢复媒体兼容目录导致的播放失败
### What was done
- 定位并修复视频加载失败：Docker 挂载的 `E:\BaiduNetdiskDownload\.deer-media-view` 被删除，导致节点容器内 `/media` 不存在。
- 重新生成 63 个视频的兼容视图并重建媒体节点，重新启动目录监听器。

### Testing
- 媒体容器 `/media` 已挂载，容器内可读取 63 个视频文件。
- `/api/v1/videos` 返回 63 条记录，当前页 50 条且可用。
- Gateway、媒体节点、Web、PostgreSQL 健康检查通过。

### Notes
- `.deer-media-view` 不是备份目录，而是 Docker 必须挂载的同盘硬链接视图；删除它会导致视频无法播放。它不复制视频内容，不应手动删除。
- `progress.md`：追加本轮故障定位和恢复记录。
- 回滚方式：无需回滚；如目录再次被删除，运行 `scripts\prepare-media-view.ps1` 后重建媒体节点即可恢复。

## 2026-07-31 - Task: 调整推荐文案与搜索分类显示
### What was done
- 将首页推荐文案统一改为“推荐”。
- 搜索词非空时隐藏工作室分类条，只保留匹配视频结果；搜索和分页接口逻辑保持不变。
- 重建并重启 Web 容器。

### Testing
- `npm.cmd run build`：通过。
- `npm.cmd run test`：4 项通过。
- `npm.cmd run test:e2e`：10 项通过。
- Web 容器健康检查：通过。

### Notes
- `web/src/App.vue`：推荐文案和搜索状态下分类条显示条件。
- `web/src/App.test.ts`、`docs/api.md`：同步文案和行为说明。
- `progress.md`：追加本轮记录。
- 回滚方式：恢复上述前端、文档文件并重新构建 Web；不涉及数据库和媒体文件。

## 2026-07-31 - Task: 解压新增视频并刷新媒体节点
### What was done
- 将 `丝米`、`师傅你是做什么工作的`、`闪闪工作室` 目录中的 16 个 7z 压缩包逐个校验后原地解压，并在成功后删除压缩包。
- 重建同盘硬链接兼容视图，使新增视频进入媒体节点扫描范围。
- 重建媒体节点容器，恢复新视频的目录同步与播放能力。

### Testing
- 三个目录剩余 `.7z`：0 个。
- 新增 MP4：16 个；源目录合计约 19.17 GB。
- 兼容目录 MP4：79 个；视频接口返回 `200` 且 `total=79`。
- `media-node`、Gateway、Web、PostgreSQL 健康状态正常。

### Notes
- `E:\BaiduNetdiskDownload\丝米`、`E:\BaiduNetdiskDownload\师傅你是做什么工作的`、`E:\BaiduNetdiskDownload\闪闪工作室`：新增视频已原地解压，原压缩包已删除。
- `E:\BaiduNetdiskDownload\.deer-media-view`：重新生成硬链接兼容视图。
- `progress.md`：追加本轮操作记录。
- 回滚方式：代码侧无需回滚；已删除的压缩包无法由代码恢复，如需恢复只能从原始下载源重新获取。

## 2026-07-31 - Task: 修复右上角提示不会自动消失
### What was done
- 为成功和错误提示增加 4 秒自动关闭计时器。
- 新提示出现时重置计时，组件卸载时清理计时器，避免提示和定时器残留。

### Testing
- `npm.cmd run test`：2 个测试文件、4 项测试通过。
- `npm.cmd run build`：通过。
- `git diff --check -- web/src/App.vue`：通过。

### Notes
- `web/src/App.vue`：统一管理提示自动关闭与卸载清理。
- `progress.md`：追加本轮记录。
- 回滚方式：移除 `noticeTimer` 和对应 `watch`、卸载清理代码，重新构建 Web。

## 2026-08-01 - Task: 管理员设置用户播放速率并移除节点带宽限速
### What was done
- 删除媒体节点总带宽令牌桶和对应环境配置，节点按实际网络、磁盘能力发送数据，仅保留媒体连接并发上限。
- 复用现有 `site_settings` 保存全局用户播放速率，默认 10 Mbps；Gateway 启动时加载，管理员保存后立即更新运行时限速。
- 新增管理员“设置”标签和播放速率表单，支持 1 到 1000 Mbps，保存后显示节点“不设上限”。
- 同步 API、部署模板、安全说明和端到端测试。

### Testing
- `go test ./...`：通过。
- `go test -race ./...`：通过。
- `go vet ./...`：通过。
- `npm.cmd run test`：4 项通过。
- `npm.cmd run build`：通过。
- `npm.cmd run test:e2e`：10 项通过。
- 管理员接口实测：读取 10 Mbps、保存 10 Mbps、再次读取 10 Mbps；数据库记录为 `user_stream_bps=10000000`。
- 节点容器环境中 `DEER_NODE_STREAM_BPS` 残留数量为 0；Gateway、媒体节点、Web、PostgreSQL 均健康。

### Notes
- `internal/store/settings.go`、`internal/httpapi/settings.go`：保存、读取和更新用户播放速率。
- `internal/httpapi/router.go`、`internal/httpapi/media.go`、`cmd/gateway/main.go`：运行时加载并应用管理员速率。
- `internal/media/node.go`、`cmd/media-node/main.go`：移除节点带宽限速链路。
- `web/src/App.vue`、`web/src/types.ts`、`web/src/styles.css`、`web/e2e/app.spec.ts`：新增设置标签、表单和验收用例。
- `compose.yaml`、`.env.example`、`deploy/`、`docs/`：同步默认值和部署说明。
- `progress.md`：追加本轮记录。
- 回滚方式：恢复上述代码/配置文件并重建三个容器；如需恢复旧用户限速，可在管理员设置中填写原值，节点限速需恢复节点代码和环境变量后重新构建。

## 2026-08-02 - Task: 固化仓库执行规范并建立首个 Git 基线
### What was done
- 将用户确认的执行、范围、验证、记录和高风险约束写入仓库根目录，后续会话可直接恢复统一施工规范。
- 补充 WireGuard 运行目录忽略规则，确保节点私钥和生成配置不会进入版本控制。
- 对当前完整项目执行后端、前端、浏览器和三套 Compose 基线验证，为首个 Git 提交建立可信落点。

### Testing
- `go test ./...`、`go test -race ./...`、`go vet ./...`：通过。
- `npm.cmd run test`：2 个测试文件、4 项测试通过。
- `npm.cmd run build`：Vue TypeScript 检查和 Vite 生产构建通过。
- `npm.cmd run test:e2e`：10 项 Playwright 测试通过。
- 本地、云端和媒体节点三套 `docker compose config --quiet`：通过。

### Notes
- `AGENTS.md`：新增仓库唯一执行规范。
- `.gitignore`：忽略所有 WireGuard 运行目录。
- `progress.md`：追加本轮基线固化和验证证据。
- 回滚方式：首个基线提交完成后可使用 `git revert` 撤销后续提交；如只撤销本轮规范落点，删除 `AGENTS.md`、恢复 `.gitignore` 并删除本条末尾追加记录。

## 2026-08-02 - Task: 增加宿主机 Caddy 隔离部署模式
### What was done
- 保留云端 Web 容器独占 `80/443` 的原部署方式，新增宿主机已有 Caddy 时使用的 Compose 覆盖配置。
- 覆盖模式只把 Web 发布到 `127.0.0.1:28200`，内部 Caddy 继续阻断内部 API、代理普通 API 并提供前端静态文件。
- 补充宿主机 Caddy 的部署、验证和回滚说明，避免与服务器现有站点和端口冲突。

### Testing
- 云端基础 Compose 与宿主机 Caddy 覆盖配置合并解析通过；最终仅发布 `127.0.0.1:28200->80/tcp` 和 WireGuard `51820/udp`，未发布容器 `443`。
- `Caddyfile.internal` 使用 Caddy 2 容器验证，返回 `Valid configuration`。
- `git diff --check`：通过。

### Notes
- `deploy/cloud/compose.host-caddy.yaml`：新增回环端口和内部 Caddy 挂载覆盖。
- `deploy/cloud/Caddyfile.internal`：新增宿主机反向代理后的内部 HTTP 配置。
- `deploy/cloud/.env.example`：新增 Web 回环端口示例。
- `docs/architecture.md`、`docs/deployment-fnos.md`：同步双层 Caddy 架构、启动验证和回滚方法。
- `progress.md`：追加本轮配置实施和验证证据。
- 回滚方式：使用 `git revert` 回滚本轮提交；服务器侧停止服务时使用基础与覆盖两份 Compose 文件且不加 `-v`，宿主机 Caddy 恢复部署前备份后 reload。

## 2026-08-02 - Task: 部署生产云端栈并接入 Windows 媒体节点
### What was done
- 在 `38.34.191.104:53612` 建立独立 `/opt/deer-screening-room` 部署目录和全新 PostgreSQL 生产卷，使用提交 `2506e14` 构建 Gateway、Web 和 WireGuard；未连接或修改其他服务器。
- 在现有宿主机 Caddy 中仅追加 `xiaolu.lwylink.xyz` 到回环 `127.0.0.1:28200` 的反向代理，保留修改前备份；104 上原有 ModelRoute、Sub2API、RelayDrive、Redis 和 MySQL 容器保持运行。
- 在当前 Windows 以独立 Compose 项目启动 WireGuard 和媒体节点，使用只读兼容视图同步 79 个视频，生产目录已可通过公网域名访问和播放。

### Testing
- 104 云端 PostgreSQL healthy、Gateway `/api/v1/health` 返回 `ok`、Web 仅监听 `127.0.0.1:28200`、WireGuard 监听 `51820/udp`。
- Caddy 候选配置和最终配置均 `Valid configuration`；`https://xiaolu.lwylink.xyz` 与 HTTP 跳转均返回 `200`。
- 104 上既有 `api.lwylink.xyz`、`sdk.lwylink.xyz`、`aggregate.lwylink.xyz` 仍返回 `200`；既有容器启动时间、运行状态和健康状态无变化。
- 本地与云端 WireGuard `wg show` 均有最新握手和传输数据；云端目录接口返回 `total=79`，视频全部 `available=true`。
- 生产业务冒烟：管理员登录成功，生成并兑换 1 鹿币，余额 `0 -> 1`；永久解锁后余额 `1 -> 0`；播放会话返回 `201`，Range 返回 `206`、`Content-Range: bytes 0-1023/1305375169`、读取 1024 字节。
- Chrome 真实浏览器打开公网首页返回 `200`，页面标题正确，渲染 50 个视频卡片；唯一控制台 `401` 为未登录首屏 `/auth/me` 的预期响应。
- 本地媒体 Compose 仅运行 `deer-screening-room-media-1` 的 WireGuard 和 media-node；原有本地业务容器状态保持不变。

### Notes
- `deploy/cloud/`：提交并部署生产 Compose、宿主机 Caddy 覆盖配置和内部 Web 配置。
- `deploy/media-node/`：使用被忽略的本地运行环境、WireGuard 私钥和封面缓存接入 Windows 媒体目录。
- `docs/architecture.md`、`docs/deployment-fnos.md`：记录宿主机 Caddy、回环端口、WireGuard 和回滚方式。
- `progress.md`：追加生产部署和真实业务验收证据。
- 回滚方式：104 上使用部署前的 `/etc/caddy/Caddyfile.pre-deer-screening-room-20260802-010217` 恢复并 `systemctl reload caddy`；停止云端使用 `docker compose -p deer-screening-room-cloud -f compose.yaml -f compose.host-caddy.yaml down`（不加 `-v`）；本地节点使用对应 Compose `down`，不删除媒体兼容视图或原始视频。

## 2026-08-02 - Task: 回档 Range 播放并收敛管理与移动端界面
### What was done
- 将工作区恢复到稳定的 HTTP Range 播放链路，保留 Artplayer、会话鉴权、Range/206 响应和节点回环部署，不再使用 WebRTC 播放入口。
- 管理后台移除 P2P/节点直连选项；播放速率设置继续保留，避免影响既有 Range 播放治理。
- 目录和已购库统一改为每页 20 条；兑换码、搜索和管理员数据的前端测试夹具同步到 20 条分页。
- 收紧鹿币页面移动端首屏：邮箱单行省略、余额区缩短、兑换表单在 320px 以上视口内可操作。

### Testing
- `go test ./...`：通过。
- `npm.cmd ci`：依赖安装完成，审计无高危命中。
- `npm.cmd run test -- --run`：2 个测试文件、4 项测试通过。
- `npm.cmd run build`：`vue-tsc` 和 Vite 生产构建通过。
- `npm.cmd run test:e2e`：11 项 Playwright 测试通过，包含桌面目录、手机两列、Range 播放页、管理页和鹿币移动端页面。
- `git diff --check`：通过；源码、配置和文档检索无 P2P/WebRTC 运行引用。

### Notes
- `internal/httpapi/catalog.go`、`web/src/App.vue`、`web/src/App.test.ts`、`web/e2e/app.spec.ts`：统一 20 条分页并更新回归夹具。
- `web/src/App.vue`、`web/src/styles.css`：移除 P2P 管理入口，优化账户邮箱、余额和兑换表单的移动端布局。
- `web/src/types.ts`、`docs/api.md`、`docs/testing.md`：同步类型、API 分页和测试说明。
- 回滚方式：恢复本轮涉及文件到当前提交；播放回档点保持 `c984c94`，生产回滚点为现有部署前备份和对应 Compose 项目，不操作其他业务容器。

## 2026-08-02 - Task: 生产回滚到 Range 播放并验收
### What was done
- 将提交 `f0cc94c` 上传到 104，保留 WebRTC 目录为回滚点；仅重建 `deer-screening-room-cloud` 的 Gateway/Web，并停止小鹿 coturn，不删除 PostgreSQL 或 WireGuard 卷。
- 本机仅重建 `deer-screening-room-media-1` 的 media-node，WireGuard、原始视频目录和其他容器保持不变；远端正式部署目录已切换为 Range 版本。
- 管理后台不再显示 P2P/节点直连入口，视频恢复为 HTTP Range + Artplayer 渐进播放。

### Testing
- 104 云端 Gateway/Web 镜像为 `f0cc94c`，PostgreSQL 与 WireGuard 持续运行；`127.0.0.1:28200/api/v1/health`、公网首页和公网健康接口均返回 200。
- 宿主机 Caddy 保持 active；`api.lwylink.xyz`、`sdk.lwylink.xyz` 仍返回 200；ModelRoute、Sub2API 等既有容器未重启。
- 生产管理员已有 6 条已购视频；真实回环业务验收：目录 `page_size=20`，播放会话 `201`，Range 首段 `206`，读取 `1024` 字节，`Content-Range: bytes 0-1023/450881632`。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、前端单测、生产构建和 11 项 Playwright 均通过。

### Notes
- `/opt/deer-screening-room/app`：当前正式 Range 代码目录；旧 WebRTC 目录按时间戳保留，可用于回滚。
- `/opt/deer-screening-room/releases/deerroom-f0cc94c.tar.gz`：本次部署归档；云端 coturn 容器已停止并移除，旧镜像/卷未做破坏性清理。
- `deploy/media-node/.env`：仅本机忽略运行配置补充已有 Relay Token 和 `DEER_VERSION=f0cc94c`，未写入 Git。
- 回滚方式：恢复 `/opt/deer-screening-room/app.webrtc-pre-range-*` 与对应备份的云端运行配置，使用旧 Compose 重新构建 Gateway/Web；本机仅恢复 media-node，PostgreSQL、WireGuard、宿主机 Caddy和其他业务不动。

## 2026-08-02 - Task: 清理本地与线上历史部署残留
### What was done
- 删除本地旧 Range 部署归档、旧 WebRTC 节点运行配置备份，以及本地生产构建和 Playwright 生成物；保留当前依赖缓存、媒体封面缓存、目录缓存和当前 WireGuard 运行配置。
- 仅在 104 的 `/opt/deer-screening-room` 内删除旧 WebRTC 代码副本、旧 WebRTC 备份、旧部署归档、重复解包发布目录和未使用 coturn 镜像；保留当前 Range 应用、当前镜像、当前发布包、Range 部署检查备份、PostgreSQL 数据卷、WireGuard 和宿主机 Caddy 配置。
- 未操作其他业务容器、其他业务 Caddy 备份、原始视频目录或媒体兼容视图。

### Testing
- 本地工作区状态干净；旧归档、旧 WireGuard 运行备份、`web/dist`、`web/test-results` 和 `web/playwright-report` 均已移除。
- 104 上小鹿 Gateway/Web/WireGuard/PostgreSQL 仍运行；ModelRoute、Sub2API 等其他容器状态未变化。
- `http://127.0.0.1:28200/api/v1/health` 和 `https://xiaolu.lwylink.xyz/api/v1/health` 均返回 `status=ok`。
- 104 上仅保留 `deerroom-app:f0cc94c`、`deerroom-web:f0cc94c` 两个小鹿镜像；当前发布包仍位于 `/opt/deer-screening-room/releases/deerroom-f0cc94c.tar.gz`。

### Notes
- `deerroom-f0cc94c.tar.gz`：删除本地旧部署归档；需要时可由当前提交重新生成 `git archive` 发布包。
- `deploy/media-node/wireguard/runtime-backups/.env.pre-webrtc-20260802-0344`：删除旧 WebRTC 节点配置备份；当前节点运行配置未改动。
- `web/dist`、`web/test-results`、`web/playwright-report`：删除本地生成物；依赖缓存 `web/node_modules` 保留供后续验证使用。
- `/opt/deer-screening-room/app.pre-webrtc-*`、`app.webrtc-pre-range-*`、`backups/webrtc-*`、`deerroom-98d4757.tar`、旧 `releases` 解包目录和未使用 coturn 镜像：已在 104 清理。
- 回滚方式：代码回滚使用当前提交重新生成发布包并仅重建 `deer-screening-room-cloud` 的 Gateway/Web；数据库、WireGuard、宿主机 Caddy和其他业务容器保持不动。当前发布包 `/opt/deer-screening-room/releases/deerroom-f0cc94c.tar.gz` 为线上代码回滚点。

## 2026-08-02 - Task: 完成 AV1 720p 全量媒体迁移
### What was done
- 使用 RTX NVENC `av1_nvenc`、CQ 30/P5 将 79 个 MP4 逐个转换为最长边 1280 的 AV1 MP4，音频统一为 AAC-LC、48 kHz、双声道、128 kbps，并启用 `+faststart`。
- 每个目标文件均通过 ffprobe 属性检查和完整 FFmpeg 解码后才原子替换并删除对应 H.264 原片；全量完成后重建 79 个硬链接兼容视图。
- 新增 Sample/Full/New 转码流程和维护锁，7z 批量解压成功后自动触发 New；媒体扫描器支持 AV1/AAC，并在缓存命中时刷新旧兼容性标记。
- 重建并启动本地 `deer-screening-room-media-1` media-node；云端目录恢复 79 条可用视频，公网 Range 播放继续工作。

### Testing
- 全量媒体审计：79/79 为 AV1 MP4，最长边不超过 1280，音频均为 AAC 双声道 48 kHz、约 126-130 kbps；无 `.av1.tmp.mp4`、`.av1-original.tmp` 或维护锁残留。
- 媒体总量由约 82.81 GiB 降至 37.33 GiB，累计时长约 43.78 小时；本轮源目录和兼容视图均为 79 个文件，源目录无竖屏素材。
- 管理员真实登录后创建播放会话，首段 Range 返回 `206`、读取 1024 字节，`Content-Range: bytes 0-1023/454416193`，内容类型为 `video/mp4`。
- WireGuard 最新握手正常，云端公开目录返回 `total=79`，首条视频为 `ready`/`av1`/`available=true`。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、前端单测、生产构建、11 项 Playwright、PowerShell 语法检查、媒体 Compose 配置检查和 `git diff --check` 均通过。

### Notes
- `scripts/transcode-av1-720p.ps1`：新增样本、全量和新增媒体转码及逐文件验证替换流程。
- `scripts/extract-7z-and-delete.ps1`：整批解压成功后自动调用新增媒体转码流程。
- `scripts/watch-media-view.ps1`：维护锁期间暂停兼容视图监听。
- `internal/media/scanner.go`、`internal/media/scanner_test.go`：支持 AV1/AAC，并覆盖旧缓存兼容性刷新回归。
- `docs/media-library.md`、`docs/local-archive-extraction.md`、`docs/operations.md`：记录 AV1 媒体格式、自动转码、维护锁和运维流程。
- 原始 H.264 文件已按用户确认的计划删除；代码可通过 Git 回滚，已删除媒体只能从独立外部备份恢复，不能由代码提交反向生成原片。

## 2026-08-02 - Task: 移除播放限速并部署到 104
### What was done
- Gateway 视频流改为不使用带宽令牌桶，媒体节点移除并发播放槽位；保留账号级播放请求频率保护（默认每分钟 120 次）。
- 管理后台移除播放限速设置入口及对应接口，生产环境删除旧限速变量；数据库仅保留兑换说明相关设置。
- 将当前工作区发布包上传到 104 的独立发布目录，仅重建小鹿 Gateway/Web；PostgreSQL、WireGuard、宿主机 Caddy、ModelRoute 和 Sub2API 未操作。

### Testing
- `go test ./...`、`go vet ./...`、前端生产构建和 `git diff --check`：通过。
- 104 本地与公网 `/api/v1/health`：均返回 200；`/api/v1/admin/settings`：返回 404。
- 线上目录返回 79 条视频，首条为 AV1/AAC、1280x720、`available=true`；Range 请求返回 `206`，`Content-Range: bytes 0-1023/454416193`。
- 线上 16 MiB Range 实测 `858145 B/s`（约 6.9 Mbps）；部署前同口径约 5.1 Mbps。
- 104 与 Windows 媒体节点 WireGuard 最新握手正常；旧业务容器运行时长和状态保持不变。

### Notes
- `internal/bandwidth/manager.go`、`internal/httpapi/settings.go`：删除已停用的带宽管理和管理设置接口；其余 Gateway/节点/前端/Compose/文档改动同步完成。
- `/opt/deer-screening-room/app.limit-removal-20260802-1`：本次线上发布目录；镜像标签为 `limit-removal-20260802`。
- `/opt/deer-screening-room/backups/.env.before-limit-removal-20260802`：部署前运行配置备份，不含在本地发布包中。
- 回滚方式：在 104 执行 `cd /opt/deer-screening-room/app && docker compose -p deer-screening-room-cloud -f deploy/cloud/compose.yaml -f deploy/cloud/compose.host-caddy.yaml up -d --no-build --no-deps gateway web`，仅恢复小鹿 Gateway/Web，不停止 PostgreSQL、WireGuard 或其他业务容器。

## 2026-08-02 - Task: 固定线上发布默认镜像标签
### What was done
- 将 104 本次发布目录的默认 `DEER_VERSION` 固定为 `limit-removal-20260802`，后续从该目录执行 Compose 重启时仍使用已验证的新 Gateway/Web 镜像。

### Testing
- Compose 配置解析出的 Gateway/Web 镜像均为 `limit-removal-20260802`；未触发容器重启。

### Notes
- `/opt/deer-screening-room/app.limit-removal-20260802-1/deploy/cloud/.env`：固定本次发布的默认镜像标签。
- 回滚点不变：使用 `/opt/deer-screening-room/app` 的旧 Range 代码和 `--no-build --no-deps gateway web` 恢复 Gateway/Web。

## 2026-08-02 - Task: 修复移动端切后台后的 Range 播放恢复
### What was done
- 停止 Artplayer 在页面后台期间自动消耗五次重连额度，改由页面恢复可见时对仍在播放的视频执行一次受控 Range 重载和播放恢复。
- 恢复流程清理播放器错误状态，并在组件卸载时移除 `visibilitychange` 监听；用户主动暂停的视频不会在回到前台后被自动启动。
- 增加播放器组件回归测试和测试文档说明。

### Testing
- `npm.cmd run test -- --run src/components/VideoPlayer.test.ts`：2 项通过，覆盖后台恢复和卸载后不再响应。
- `npm.cmd run test -- --run`：3 个测试文件、6 项通过。
- `npm.cmd run build`：Vue 类型检查和 Vite 生产构建通过。
- `git diff --check`：通过。

### Notes
- `web/src/components/VideoPlayer.vue`：增加移动端页面可见性恢复、错误状态清理和受控播放重载。
- `web/src/components/VideoPlayer.test.ts`：新增 Artplayer mock 与后台恢复回归测试。
- `docs/testing.md`：记录后台恢复测试边界。
- `progress.md`：追加本轮修复和验证证据。
- 回滚方式：恢复上述三个代码/文档文件到本轮前版本；不涉及 Gateway、媒体节点、数据库或其他业务容器。

## 2026-08-02 - Task: 部署移动端后台恢复修复
### What was done
- 将本轮 Range 播放恢复修复打包上传到 104 的独立发布目录，仅重建 `deer-screening-room-cloud` 的 Web 容器。
- 新 Web 镜像使用 `range-resume-20260802` 标签并切换到 `/opt/deer-screening-room/app.range-resume-20260802-2`；运行配置权限保持受限，未进入发布包或日志。
- Gateway、PostgreSQL、WireGuard、宿主机 Caddy 及 ModelRoute/Sub2API 等其他业务未重启或修改。

### Testing
- 远端 Compose 配置校验和 Web Docker 构建通过，镜像内 Vue 生产构建通过。
- Web 容器已运行且公网 `https://xiaolu.lwylink.xyz/` 返回 `200`，`/api/v1/health` 返回 `200`，HTTP 自动跳转返回 `308`。
- 回环健康接口返回 `status=ok`；宿主机 Caddy 保持 `active`，`28200` 仍为 Web 回环监听。
- 部署前后 Gateway、PostgreSQL、WireGuard、ModelRoute 和 Sub2API 容器 ID/运行状态保持不变；公网 HTML 已引用新资源 `assets/index-Ce9j1pRK.js`。

### Notes
- `/opt/deer-screening-room/releases/deerroom-range-resume-20260802.zip`：本轮源码发布包，不含运行密钥。
- `/opt/deer-screening-room/app.range-resume-20260802-2`：线上新 Web 发布目录。
- `progress.md`：追加生产部署证据和回滚点。
- 回滚方式：在 104 使用旧发布目录 `/opt/deer-screening-room/app.limit-removal-20260802-1` 的 Compose 配置执行 `docker compose -p deer-screening-room-cloud -f deploy/cloud/compose.yaml -f deploy/cloud/compose.host-caddy.yaml up -d --no-build --no-deps web`，仅恢复 Web 容器；Gateway、数据库、WireGuard 和其他业务不动。

## 2026-08-02 - Task: 创建 GitHub 私有仓库并接入 GHCR
### What was done
- 在 GitHub 账号 `lwy183178053` 下创建私有仓库 `deer-screening-room`，将本地 `main` 发布到远程并保留当前播放器修复提交。
- 新增 GitHub Actions 容器发布流程，使用 `v*` 标签构建 `linux/amd64` 媒体节点镜像并推送到 `ghcr.io/lwy183178053/deer-screening-room`。
- 新增 NAS registry Compose 覆盖配置，绿联 Docker 项目可直接拉取固定镜像标签，继续通过运行时环境注入节点凭据和媒体路径。

### Testing
- `docker compose --env-file deploy/media-node/.env.example -f deploy/media-node/compose.yaml -f deploy/media-node/compose.registry.yaml config --quiet`：通过。
- GitHub Actions run `30744597658`：完成且结论为 `success`，标签 `v0.1.0` 已触发镜像发布。
- GitHub 仓库 API：默认分支为 `main`，仓库保持私有。
- `git diff --check`：通过。

### Notes
- `.github/workflows/publish-container.yml`：GHCR 自动构建和发布工作流。
- `deploy/media-node/compose.registry.yaml`：跳过本地构建并拉取 GHCR 镜像的 Compose 覆盖。
- `deploy/media-node/.env.example`、`docs/deployment-fnos.md`：补充镜像版本和绿联 NAS 拉取说明。
- `progress.md`：追加仓库与镜像发布证据。
- 回滚方式：NAS 将 `DEER_VERSION` 指向历史 GHCR 标签后执行 `docker compose pull` 和 `up -d --no-build`；代码侧使用 Git 历史提交恢复工作流。

## 2026-08-02 - Task: 收敛媒体节点为面板环境变量配置
### What was done
- 新增单文件 `compose.ugreen.yaml`，媒体节点镜像、WireGuard 镜像、节点名称、地址、公钥、Token、Gateway 端点、扫描周期和海报目录全部由环境变量注入。
- 新增绿联环境变量模板，媒体目录使用唯一的 `/CHANGE_ME` 占位路径；同一模板可复制给其他节点使用。
- 保留本地开发 Compose 和 registry 覆盖配置，绿联项目可直接导入单文件并使用已导入的 amd64 镜像。

### Testing
- `docker compose --env-file deploy/media-node/.env.ugreen.example -f deploy/media-node/compose.ugreen.yaml config --quiet`：待执行。
- `docker compose --env-file deploy/media-node/.env.example -f deploy/media-node/compose.yaml -f deploy/media-node/compose.registry.yaml config --quiet`：待执行。
- `git diff --check`：待执行。

### Notes
- `deploy/media-node/compose.ugreen.yaml`：绿联 Docker 单文件项目配置。
- `deploy/media-node/.env.ugreen.example`：节点差异环境变量模板。
- `deploy/media-node/compose.yaml`、`compose.registry.yaml`、`.env.example`：统一镜像、WireGuard 和目录变量。
- `docs/deployment-fnos.md`：更新单文件导入与多节点复制说明。
- `progress.md`：追加本轮配置收敛记录。
- 回滚方式：绿联项目切回上一版 Compose 文件或停止该项目；云端节点凭据和 Windows 节点运行状态保持原样。

## 2026-08-03 - Task: 节点单一身份与短命名部署基线
### What was done
- Gateway 移除静态节点凭据映射和单节点 Token fallback，节点凭据改为数据库加密保存并按节点生成一次性 Docker 安装包。
- 管理员节点页增加创建、一次性下载、凭据轮换和删除流程；删除会撤销 WireGuard Peer，并清理节点目录索引、视频权益和播放会话，源视频文件保留。
- 新增 WireGuard Peer 应用器和独立 `/app/wireguard-provisioner` 入口，安装包与媒体节点 Compose 统一使用 `deer`、`deer-node`、`deer-wg`、`deer-wg-init` 短命名。
- 安装包只需在 Docker 项目环境中设置 `DEER_MEDIA_HOST_PATH`；同步更新云端、绿联部署文档和安全说明，移除旧的第二节点模板及本轮冗余代码。

### Testing
- `go test ./...`、`go test -race ./...`、`go vet ./...`：通过。
- `npm.cmd run test -- --run`：3 个测试文件、6 项通过；`npm.cmd run build`：通过。
- 根目录、云端宿主 Caddy 覆盖、绿联节点和 registry 覆盖 Compose `config --quiet`：通过。
- `git diff --check`：通过；敏感变量扫描未命中已跟踪文件。
- `docker build -t deerroom-app:node-provisioning-20260803 .`：通过；镜像内三个入口文件均存在。
- 104 只读盘点：小鹿现有 Gateway/Web/PostgreSQL/WireGuard 正常运行，宿主机 Caddy active，`28200` 为回环监听；ModelRoute、Sub2API 及其数据库/Redis 容器未操作。

### Notes
- `internal/httpapi/node_provisioning.go`、`internal/provisioning/`、`internal/wireguard/`、`cmd/wireguard-provisioner/`：节点创建、凭据封装、安装包和 WireGuard Peer 应用。
- `internal/store/migrations/004_node_provisioning.sql`、`internal/store/catalog.go`、`internal/store/types.go`：节点身份字段、地址复用和级联删除。
- `deploy/cloud/compose.yaml`、`deploy/cloud/compose.host-caddy.yaml`、`deploy/media-node/compose.ugreen.yaml`、`internal/provisioning/provisioning.go`：短命名和部署模板。
- `web/src/App.vue`、`web/src/styles.css`、`web/src/types.ts`：管理员节点操作和移动端适配。
- `docs/architecture.md`、`docs/deployment-fnos.md`、`docs/security.md`：架构、安装和凭据生命周期说明。
- 回滚点：当前本地提交；线上发布前保留 104 原 `/opt/deer-screening-room/app.range-resume-20260802-2`、旧 Compose 文件、Caddyfile 和 WireGuard 配置备份。云端异常时仅恢复小鹿 Gateway/Web，节点异常时仅停止 `deer-node` 项目，不使用 `-v`，不触碰其他业务。

## 2026-08-03 - Task: 部署首个短命名节点并完成切换
### What was done
- 将提交版本和镜像发布到 104 的独立目录 `/opt/deer-screening-room/app.node-provisioning-20260803-1`，保留旧发布目录、运行环境、WireGuard 配置和宿主机 Caddyfile 备份。
- 104 仅重建小鹿 Gateway、Web、WireGuard 和新增的 `wireguard-provisioner`；PostgreSQL 数据卷复用但未清空，ModelRoute、Sub2API、Redis、MySQL 和宿主机 Caddy 未停止或修改。
- 在管理员节点页删除旧节点并创建 `windows-media`，自动复用 `10.77.0.2/32`，下载并轮换一次性安装包；删除动作清理旧目录索引、视频权益和播放会话，源视频保留。
- 当前 Windows 仅停止旧小鹿 media-node/WireGuard 容器，启动 `deer-node`/`deer-wg`/`deer-wg-init`；新增 WireGuard 健康检查，节点等待隧道就绪后再扫描，避免首启竞态。
- 修正安装包对 PostgreSQL `inet` 的 `/32` 规范化和 Compose YAML 环境字段，安装包可解析且只需设置媒体目录变量。

### Testing
- 本地 `go test ./...`、`go test -race ./...`、`go vet ./...`：通过。
- 前端测试 3 个文件、6 项通过；生产构建通过。
- 根目录、云端宿主 Caddy 覆盖、绿联节点和 registry 覆盖 Compose `config --quiet`：通过；`git diff --check`：通过。
- 104 Gateway/Web 健康接口、公网 `https://xiaolu.lwylink.xyz/api/v1/health`、公网首页 HTTPS 和 Caddy `validate`：通过。
- 104 节点页：`windows-media` 在线、WireGuard `10.77.0.2/32`、扫描 `ok`；目录总量 79，分页首屏 20，首条视频为 AV1/AAC。
- 真实管理员会话完成一次解锁、播放会话和 Range 验证：HTTP `206 Partial Content`，读取 1024 字节，`Content-Range: bytes 0-1023/454416193`。
- 当前电脑 `deer-wg` 健康且与 `38.34.191.104:51820` 有最新握手；`deer-node` 健康接口返回成功。ModelRoute、Sub2API、Redis、MySQL 容器状态保持运行。

### Notes
- `/opt/deer-screening-room/app.node-provisioning-20260803-1`：当前 104 小鹿发布目录；`/opt/deer-screening-room/backups/node-provisioning-before-20260803`：部署前备份目录。
- `deploy/media-node/compose.yaml`、`deploy/media-node/compose.ugreen.yaml`、`internal/provisioning/provisioning.go`：健康检查、地址规范化和安装包 Compose 模板。
- 当前电脑旧节点容器与空项目网络已删除；旧绑定 WireGuard 配置目录仍作为本地忽略的回滚副本保留，未与新 `deer_wireguard-data` 卷混用。原始媒体目录和 poster 缓存未删除。
- 回滚方式：104 仅使用 `/opt/deer-screening-room/app.range-resume-20260802-2` 的 Compose 文件和备份 WireGuard/Caddy 配置恢复 Gateway/Web；当前电脑仅停止 `deer` 项目并恢复旧 media-node Compose，不使用 `-v`，不触碰其他业务容器。

## 2026-08-03 - Task: 修复 NAS 节点删除 502 并完成删除
### What was done
- 修复 Gateway 读取 PostgreSQL `inet` 地址后携带 `/32` 调用 WireGuard provisioner 的问题，撤销前统一转换为主机地址，避免 provisioner 返回 `peer_remove_failed`。
- 在 104 仅切换 Gateway 和 `wireguard-provisioner` 到修复镜像；Web、WireGuard、PostgreSQL、ModelRoute、Sub2API 及其 Redis/MySQL 未重启或修改。
- 正式删除节点 `nas-media-1`（ID `2549`），接口返回 `204`；云端数据库节点记录、该节点 Peer 和相关级联数据已清理，`windows-media`（ID `2423`）保留并继续握手。

### Testing
- `go test ./internal/httpapi ./internal/provisioning ./internal/wireguard`：通过，包含 `/32` 地址归一化回归测试。
- `docker build --tag deerroom-app:node-provisioning-20260803-5 .`：本地通过；104 本机同标签镜像构建通过。
- `https://xiaolu.lwylink.xyz/api/v1/health`：返回 `200`；管理员删除请求：`204 No Content`。
- 104 数据库仅剩 `windows-media`；WireGuard 配置和运行状态仅剩 `10.77.0.2/32`，最新握手正常；小鹿和其他业务容器状态保持运行。

### Notes
- `internal/httpapi/node_provisioning.go`、`internal/httpapi/node_provisioning_test.go`：删除节点时归一化 `inet` CIDR 地址并增加回归测试。
- `docs/deployment-fnos.md`、`docs/operations.md`：补充节点删除清理范围和源视频保留说明。
- 104 当前发布目录：`/opt/deer-screening-room/app.node-provisioning-20260803-5`；旧目录 `/opt/deer-screening-room/app.node-provisioning-20260803-1` 和旧镜像仍保留。
- 回滚点：仅将小鹿 Compose 的 `DEER_VERSION` 恢复为 `node-provisioning-20260803-4` 并重建 Gateway/provisioner；不使用 `-v`，不操作 `windows-media` 或其他业务容器。

## 2026-08-03 - Task: 移除媒体兼容视图并回退 Windows 节点测试
### What was done
- 移除媒体节点的 `/source -> media-view` 同步、`catalog-titles.json` 标题清单、兼容视图实现和 Windows 监听器脚本；节点现在直接把原始目录只读挂载到 `/media`。
- 更新节点安装包模板、Windows/Linux/NAS Compose、转码脚本和媒体文档，不再生成、挂载或重建兼容视图；修正 registry Compose 覆盖中的无效 `build: null`。
- 本机停止视图监听器，删除 `E:\BaiduNetdiskDownload\.deer-media-view` 和 `deer_media-view` 卷，保留原始视频、`deer_wireguard-data` 和其他业务容器。
- 本机节点切换到 `ghcr.io/lwy183178053/deer-screening-room:v0.1.0`，用于直接读取原始 Windows 目录的回归测试。

### Testing
- `go test ./...`：通过。
- Windows、绿联和 registry Compose `config --quiet`：通过。
- `deer-node` 健康接口返回 `200`；直接扫描日志复现 `readdirent /media/悠米: input/output error`，当前只能枚举到 42 个 MP4，证明问题发生在 Docker 枚举目录阶段而非网页标题乱码。
- 原始目录文件名审计：8 个文件名 UTF-8 长度超过 255 字节，最长 304 字节，均位于 `悠米` 目录；原始文件未重命名、复制或删除。

### Notes
- `internal/media/node.go`、`internal/media/scanner.go`、`internal/config/config.go`、`cmd/media-node/main.go`：删除兼容视图和标题清单读取。
- `internal/media/view.go`、`internal/media/view_test.go`、`scripts/prepare-media-view.ps1`、`scripts/watch-media-view.ps1`、`scripts/watch-media-view.cmd`：删除不再使用的视图实现和监听器。
- `deploy/media-node/*`、`internal/provisioning/provisioning.go`、`scripts/transcode-av1-720p.ps1`、`docs/*`：改为直接挂载原始目录并同步说明。
- 回滚点：本机将 `deploy/media-node/.env` 的 `DEER_VERSION` 改回 `v0.1.1` 并恢复视图代码提交；恢复前不要删除原始目录。当前 `v0.1.0` 测试节点可单独停止，不影响 `deer-wg` 和其他容器。

## 2026-08-03 - Task: 规划媒体哈希文件名与标题映射迁移
### What was done
- 明确保留工作室目录、仅将视频文件改为短哈希名，并在媒体根目录维护 `.deer-media-map.json` 恢复原始显示标题。
- 明确 Windows Docker 无法在容器内枚举超长文件名，计划由 Windows 宿主机预处理器完成首次和新增文件标准化，Linux/NAS 节点处理可枚举的新文件。
- 形成设计说明和分阶段实施计划，包含单文件测试、样本迁移、79 个文件全量迁移、缓存清理、节点验证和回滚点。

### Testing
- 仅完成计划文档自检：无代码或媒体文件改动；`git diff --check` 通过。
- 实施前必须按计划先运行失败测试，再进入代码和实际文件迁移。

### Notes
- `docs/superpowers/specs/2026-08-03-media-hash-names-design.md`：记录映射格式、平台边界和回滚约束。
- `docs/superpowers/plans/2026-08-03-media-hash-names.md`：记录分任务实现、测试、部署和迁移步骤。
- `progress.md`：追加本轮计划落点。
- 回滚方式：本轮无运行代码和媒体文件改动；删除本轮计划文档即可回到上一提交。实施阶段按计划使用映射表恢复文件名并恢复旧 Compose/镜像。

## 2026-08-03 - Task: 固化哈希媒体键并更新本机节点
### What was done
- 为媒体映射补充 `original_path`，扫描器按原始规范化相对路径生成稳定媒体键，避免视频文件改名后产生重复目录记录。
- 为现有 Windows 媒体根目录 79 条映射补齐 `original_path`；79 个视频仍为哈希文件名，未复制、转码、删除或改变工作室目录。
- 重建本机 `deerroom-app:media-hash-20260803` 并仅重建 `deer-node`；现有 `deer-wg`、ModelRoute、Redis、MySQL 等容器未重启。
- 更新媒体库设计文档，说明映射字段和媒体键稳定性。

### Testing
- `go test ./...`、`go test -race ./...`、`go vet ./...`：通过。
- `npm.cmd test`：3 个测试文件、6 项通过；`npm.cmd run build`：生产构建通过。
- 根 Compose、云端 Compose、宿主 Caddy 覆盖、Windows 节点、绿联节点和 registry 覆盖 `config --quiet`：通过；`git diff --check`：通过。
- Windows 媒体审计：视频 79、哈希文件 79、映射 79、缺失 `original_path` 0、超长文件名 0。
- 本机 `deer-node` `/api/v1/health` 返回 200；`deer-wg` 与云端端点保持最新握手；公网目录返回总数 79，AV1/AAC 视频均为 ready。

### Notes
- `internal/media/name_map.go`、`internal/media/scanner.go`：记录原始路径并使用稳定媒体键。
- `internal/media/name_map_test.go`、`internal/media/scanner_test.go`：覆盖映射往返、缓存命中和改名后媒体键不变。
- `scripts/normalize-media-names.ps1`、`scripts/normalize-media-names.tests.ps1`：升级旧映射并验证幂等行为。
- `docs/media-library.md`、`docs/superpowers/specs/2026-08-03-media-hash-names-design.md`：同步稳定键说明。
- `E:\BaiduNetdiskDownload\.deer-media-map.json`：现有 79 条记录已补齐原始路径字段，不纳入 Git。
- 回滚点：代码回退到提交 `ffdf533` 后重新构建并仅重启 `deer-node`；映射中的新增字段旧版本会忽略，原始视频文件仍保留。

## 2026-08-03 - Task: 发布最新节点镜像并修正节点包默认版本
### What was done
- 将稳定媒体键修复镜像发布为 `ghcr.io/lwy183178053/deer-screening-room:v0.1.2`，并同步更新 `latest` 标签；两者指向同一 amd64 镜像摘要。
- 将本地开发 Compose、云端环境示例和节点包默认版本统一为 `latest`，避免 Web 下载的 `node.env` 固定到旧版 `v0.1.0`。
- 备份 104 当前小鹿云端环境文件，只更新 `DEER_NODE_VERSION` 为 `latest`，并仅重建 Gateway；PostgreSQL、WireGuard、Web 和其他业务容器保持运行。

### Testing
- GHCR `v0.1.2` 与 `latest` 均可通过镜像元数据检查，平台为 `linux/amd64`，摘要为 `sha256:6ead051bd79eb98e06495e3cda71e943a6b64283fc0f32e1bc1928524696d9e4`。
- `go test ./internal/config ./internal/provisioning ./internal/httpapi`：通过；根 Compose 与云端 Compose `config --quiet`：通过；`git diff --check`：通过。
- 104 Gateway 环境确认 `DEER_NODE_VERSION=latest`，健康接口返回成功；其他小鹿容器状态保持运行。

### Notes
- `compose.yaml`：本地 Gateway 默认节点版本改为 `latest`。
- `deploy/cloud/.env.example`：云端配置示例改为 `DEER_NODE_VERSION=latest`。
- `progress.md`：记录镜像发布和 104 配置更新证据。
- 104 运行目录 `/opt/deer-screening-room/app.node-provisioning-20260803-5/deploy/cloud`：保留 `.env.before-node-version-20260803` 作为回滚副本。
- 回滚点：将 104 `.env` 的 `DEER_NODE_VERSION` 恢复为备份值并仅重建 Gateway；镜像回滚可使用既有 GHCR 历史标签。

## 2026-08-03 - Task: 清理历史缓存残留
### What was done
- 对节点 `posters` 缓存与当前 `catalog-cache.json` 做引用比对，删除 85 个未被 79 条现行媒体记录引用的旧 JPG，共释放约 1.6 MB。
- 删除 `internal/auth`、`internal/bandwidth`、`internal/zpay` 三个空目录；未删除历史进度、迁移文件、部署模板或业务代码。

### Testing
- 封面缓存核验：目录缓存 79 条、JPG 79 个、缺失引用 0、未引用文件 0。
- `go test ./...`：通过；根、云端和绿联 Compose `config --quiet`：通过；`git diff --check`：通过。

### Notes
- `deploy/media-node/posters/*.jpg`：仅删除未被当前目录缓存引用的历史封面，现行封面全部保留。
- `internal/auth`、`internal/bandwidth`、`internal/zpay`：删除空目录，不包含可执行代码。
- `progress.md`：记录清理范围和验证结果。
- 回滚方式：现行封面缺失时执行管理员重新扫描，节点会按现有扫描逻辑重新生成；本轮未改变视频、数据库或播放逻辑。

## 2026-08-03 - Task: 管理节点显示工作室与视频统计
### What was done
- 管理节点接口现在返回每个节点当前可用且已发布的视频数量，以及这些视频所属的已发布工作室数量。
- 节点卡片新增工作室、视频、总容量和可用容量四项统计；移动端在窄屏下保持两列布局。
- 补充节点统计查询的集成测试，并同步更新管理员 API 文档。

### Testing
- `go test ./...`：通过。
- `go test -race ./...`：通过。
- `go vet ./...`：通过。
- `web`: `npm.cmd run test -- --run`（3 个测试文件、6 项通过）和 `npm.cmd run build`：通过。
- 根 Compose、云端 Compose、宿主 Caddy 组合覆盖、Windows 节点、registry 节点和绿联节点 Compose `config --quiet`：通过；宿主 Caddy 覆盖按与基础云端 Compose 组合方式检查。
- `git diff --check`：通过。
- 公网健康接口返回 `200`，公开目录返回 AV1/AAC、1280×720 且 `ready` 的视频；未使用管理员会话直接读取节点明细，因此 NAS 节点的实时计数仍需在管理页刷新确认。

### Notes
- `internal/store/types.go`、`internal/store/catalog.go`：增加节点统计字段并从现有目录表聚合查询。
- `internal/store/postgres_integration_test.go`：覆盖可用/已发布视频和不兼容视频的统计口径，并校验 `ListNodes`、`NodeByID`。
- `web/src/types.ts`、`web/src/App.vue`、`web/src/styles.css`：展示节点工作室/视频数量并适配移动端两列布局。
- `docs/api.md`：记录节点统计字段。
- `progress.md`：追加本轮实施与验证记录。
- 回滚点：回退本轮提交即可移除统计字段和卡片展示；数据库表、视频文件、节点凭据和播放逻辑均未改变。

## 2026-08-03 - Task: 部署节点统计到 104
### What was done
- 将提交 `753181d` 归档上传到 104 的独立发布目录 `/opt/deer-screening-room/app.node-counts-20260803-1`，复制运行配置并将新镜像标签固定为 `node-counts-20260803-1`。
- 仅重建并切换小鹿 `gateway` 和 `web`；PostgreSQL、WireGuard、WireGuard provisioner、宿主 Caddy、ModelRoute、Sub2API 及其数据库/Redis 未重启或修改。
- 统计查询确认 `nas-media-1` 当前在线但为 0 个工作室、0 个视频；`windows-media` 当前在线，有 5 个工作室、79 个视频。

### Testing
- 远端 Gateway/Web 镜像源码构建通过，Vue 生产构建通过。
- 回环 `http://127.0.0.1:28200/api/v1/health`、公网 `/api/v1/health` 和首页 HTTPS 均返回成功（`200`）。
- 公网视频目录返回 `79` 条，目录内容保持 AV1/AAC、`ready`。
- 104 宿主 Caddy 保持 `active`；`80/443` 仍由宿主 Caddy 监听，Web 仍只绑定 `127.0.0.1:28200`。
- PostgreSQL、WireGuard 和 provisioner 容器启动时间与部署前保持不变；其他业务容器状态保持运行。
- Caddyfile 部署前后 SHA-256 一致；未执行 Caddy reload。

### Notes
- `/opt/deer-screening-room/app.node-counts-20260803-1`：本轮线上发布目录；Gateway/Web 镜像为 `deerroom-app:node-counts-20260803-1` 和 `deerroom-web:node-counts-20260803-1`。
- `/opt/deer-screening-room/backups/node-counts-before-20260803-1`：部署前 `.env`、Caddyfile、容器状态和 inspect 备份，不含在 Git 中。
- `progress.md`：追加线上部署证据和 NAS 统计结果。
- 回滚方式：仅恢复旧发布目录的 Gateway/Web：Gateway 使用 `/opt/deer-screening-room/app.node-provisioning-20260803-5/deploy/cloud`，Web 使用 `/opt/deer-screening-room/app.node-provisioning-20260803-1/deploy/cloud`，分别执行 Compose `up -d --no-build --no-deps gateway` 与 `up -d --no-build --no-deps web`；不使用 `-v`，不触碰数据库、WireGuard 或其他业务。

## 2026-08-03 - Task: 项目整理与潜在问题修复
### What was done
- 对 Go、媒体节点、节点安装包、脚本、前端和 Compose 配置做了静态检查，确认当前播放链路仍为 HTTP Range，未发现活跃的 WebRTC、兼容视图或 `/source` 业务残留。
- 修复 PostgreSQL 多进程同时启动时的迁移竞态：`Migrate()` 现在在同一连接上使用数据库 advisory lock，保留原有逐迁移事务和数据库结构。
- 修复 AV1 转码维护脚本查找旧服务名的问题，维护阶段现在实际停止并恢复当前 Compose 的 `node` 服务；同步修正运维文档中的服务名。
- 将节点示例配置中的项目名和默认镜像版本统一为 `deer`/`latest`，避免新节点包继承历史长项目名或旧版本。

### Testing
- 未加锁时两个独立进程并发初始化全新 PostgreSQL 稳定复现 `pg_type_typname_nsp_index` 冲突；加锁后同样场景不再出现迁移冲突。
- 使用独立全新数据库运行 `internal/store` 和 `internal/httpapi` 集成测试：均通过；并发共享同一测试库时仅出现测试数据互相清空导致的断言失败，测试文档已要求使用不同数据库 URL。
- `go test ./...`、`go test -race ./...`、`go vet ./...`：通过。
- `web`: Vitest 3 个文件/6 项通过，生产构建通过；Playwright 11 项通过。
- `scripts/normalize-media-names.tests.ps1`：通过；转码脚本 PowerShell 解析通过。
- 根 Compose、云端 Compose、宿主 Caddy 覆盖、Windows 节点和绿联节点 Compose `config --quiet`：通过；`git diff --check`：通过。

### Notes
- `internal/store/postgres.go`：增加跨进程迁移锁；回滚点为移除本轮 advisory lock 代码并恢复原 `Migrate()` 实现。
- `scripts/transcode-av1-720p.ps1`：将维护脚本服务名从历史 `media-node` 改为当前 `node`。
- `docs/operations.md`：同步当前转码维护服务名。
- `deploy/media-node/.env.example`、`deploy/media-node/.env.ugreen.example`：统一短项目名和 `latest` 默认版本。
- `progress.md`：记录本轮检查、修复和验证证据。
- 回滚方式：回退本轮整理提交即可；不需要删除数据库、媒体文件、节点凭据或线上备份。本轮未部署 104，也未重启任何线上容器。

## 2026-08-03 - Task: 部署项目整理修复到 104
### What was done
- 将提交 `f3e99e0` 归档上传到独立发布目录 `/opt/deer-screening-room/app.audit-20260803-1`，复制现行云端运行配置和 WireGuard 配置。
- 在 104 构建 `deerroom-app:project-audit-20260803-1`，仅重建小鹿 Compose 项目的 `gateway` 服务；Web、PostgreSQL、WireGuard、WireGuard provisioner 及其他业务容器保持原容器运行。
- 迁移锁修复已在线生效，数据库迁移记录为 5 条；两个媒体节点均保持在线。

### Testing
- 104 回环 `http://127.0.0.1:28200/api/v1/health` 返回 `200`；`https://xiaolu.lwylink.xyz/api/v1/health` 返回 `200`。
- 公网目录请求返回 79 条视频；节点状态查询显示 `nas-media-1` 和 `windows-media` 在线。
- 新 Gateway 镜像为 `deerroom-app:project-audit-20260803-1`，Web 仍为 `deerroom-web:node-counts-20260803-1`。
- Web、PostgreSQL、WireGuard、provisioner 与 ModelRoute/Sub2API 容器启动时间保持部署前记录；未执行 Caddy reload。

### Notes
- `/opt/deer-screening-room/app.audit-20260803-1`：本轮线上发布目录。
- `/opt/deer-screening-room/backups/project-audit-before-20260803-1`：部署前环境文件、Gateway inspect 和容器状态备份，不含在 Git 中。
- `progress.md`：追加线上部署证据和回滚点。
- 回滚方式：在 104 使用旧发布目录 `/opt/deer-screening-room/app.node-counts-20260803-1/deploy/cloud`，执行 `DEER_VERSION=node-counts-20260803-1 docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml up -d --no-build --no-deps gateway`；不使用 `-v`，不操作 Web、数据库、WireGuard 或其他业务容器。

## 2026-08-03 - Task: 迁移 MP4 元数据标题并部署本机 v0.1.3 节点
### What was done
- 将节点媒体识别从根目录 JSON 映射迁移到 MP4 元数据：`title` 用于网页标题，`deer_media_key` 继续保存原有稳定身份；扫描器不再改写媒体目录。
- 新增 Windows 元数据整理脚本，使用 FFmpeg 流复制写入元数据，按 240 个 UTF-8 字节安全截断超长文件名并追加 8 位哈希；AV1 转码与 7z 解压流程已接入该处理。
- 在媒体根目录外创建 37.33 GiB 完整备份和迁移快照，普通/超长双样本通过后迁移 79 个 MP4；旧 JSON 已从真实媒体根目录删除，原工作室、79 个媒体键和 2 条购买权益保持。
- 节点媒体挂载统一改为只读，节点包和缺省配置固定使用 `v0.1.3`；本机仅替换三个已停止的小鹿节点容器，并以 `deerroom-app:v0.1.3` 重新上线。

### Testing
- `go test ./...`、`go test -race ./...`、`go vet ./...`：通过。
- `web`: Vitest 3 个文件/6 项通过，生产构建通过，Playwright 11 项通过。
- `scripts/set-media-metadata.tests.ps1`：通过超长 Unicode、240 字节边界、幂等和目标冲突保护；4 个 PowerShell 脚本解析通过。
- 根 Compose、云端 Compose、宿主 Caddy 组合、Windows 节点、registry 节点和绿联节点共 6 套配置解析通过；`git diff --check` 通过。
- 迁移前旧映射、节点缓存和线上 `windows-media` 可用目录均为相同的 79 个媒体键；外部备份 81 个文件逐文件 SHA-256 一致。
- 双样本与全量验收均保持标题、稳定键、时长、轨道和工作室；迁移后为 79 个唯一键、0 个哈希媒体文件名、0 个超过 240 字节文件名、0 个迁移临时文件。
- 本机 `deer-wg` 健康且有最新握手，`deer-node` 使用 `v0.1.3`，`/media` 为只读；线上节点为 79 个可用视频、5 个工作室、2 条权益，哈希标题计数为 0。
- 节点真实媒体请求返回 `206 Partial Content`，`Content-Range` 为 `bytes 0-1023/558673160`，响应体为 1024 字节；公网健康接口返回成功。
- 104 的 Web、PostgreSQL、WireGuard、provisioner、ModelRoute 和 Sub2API 启动时间保持本轮操作前记录。本机其他业务容器未被本轮命令操作；其启动时间在小鹿节点重建前由外部状态变化更新，已按实际情况保留记录。

### Notes
- `compose.yaml`：媒体挂载改为只读，节点包缺省版本固定为 `v0.1.3`。
- `deploy/cloud/.env.example`、`deploy/cloud/compose.yaml`：云端节点包版本示例和缺省值固定为 `v0.1.3`。
- `deploy/media-node/.env.example`、`deploy/media-node/.env.ugreen.example`：节点示例镜像版本固定为 `v0.1.3`。
- `deploy/media-node/compose.yaml`、`deploy/media-node/compose.ugreen.yaml`：节点媒体目录挂载改为只读。
- `internal/config/config.go`、`internal/config/config_test.go`：Gateway 的节点包缺省版本固定为 `v0.1.3` 并增加回归测试。
- `internal/media/scanner.go`、`internal/media/scanner_test.go`：读取 MP4 标题和稳定键元数据，按相对路径命中缓存，不再写媒体目录。
- `internal/media/name_map.go`、`internal/media/name_map_test.go`：删除运行时 JSON 映射实现及测试。
- `internal/provisioning/provisioning.go`、`internal/provisioning/provisioning_test.go`：生成只读媒体挂载、固定版本且不依赖 JSON 映射的节点包。
- `scripts/set-media-metadata.ps1`、`scripts/set-media-metadata.tests.ps1`：新增流复制元数据迁移、文件名截断、校验和回归测试。
- `scripts/transcode-av1-720p.ps1`、`scripts/extract-7z-and-delete.ps1`：转码直接写入元数据，整批处理后统一整理，不再调用旧标准化脚本。
- `scripts/normalize-media-names.ps1`、`scripts/normalize-media-names.tests.ps1`：删除旧哈希改名脚本及测试。
- `docs/architecture.md`、`docs/deployment-fnos.md`、`docs/local-archive-extraction.md`、`docs/media-library.md`、`docs/operations.md`、`docs/security.md`：同步元数据身份、只读挂载、自动处理和回滚说明。
- `docs/superpowers/plans/2026-08-03-media-hash-names.md`、`docs/superpowers/specs/2026-08-03-media-hash-names-design.md`：删除已废弃的 JSON/哈希文件名设计。
- `progress.md`：追加本轮实施、验证、生产基线与回滚记录。
- `E:\BaiduNetdiskDownload.metadata-backup-20260803`：仓库外完整原片、旧映射和 `migration-snapshot.json` 备份；不纳入 Git。
- 回滚方式：先执行 `docker compose --project-name deer-screening-room-media-1 --file deploy/media-node/compose.yaml --env-file deploy/media-node/.env down`（不使用 `-v`），将当前媒体目录移到隔离位置后从 `E:\BaiduNetdiskDownload.metadata-backup-20260803` 恢复原目录；代码回退到 `c04f52e`，本机节点固定旧镜像 `media-hash-20260803` 后重新启动。104 尚未在本条记录阶段更新。

## 2026-08-03 - Task: 发布 v0.1.3 镜像并部署 104 Gateway
### What was done
- 推送 `codex/webrtc-p2p` 的功能提交与 `v0.1.3` 标签，通过现有 GitHub Actions 发布节点镜像，并同步更新 `latest`。
- 在 104 建立独立发布目录和部署前备份，复用现行运行配置，将节点安装包版本固定为 `v0.1.3`；仅重建小鹿 Gateway。
- Web、PostgreSQL、WireGuard、WireGuard provisioner、宿主机 Caddy、ModelRoute、Sub2API 及其数据库和 Redis 均未重启或修改。

### Testing
- GitHub Actions 发布任务成功；`v0.1.3` 与 `latest` 的 OCI 摘要均为 `sha256:e73eda2b0912f4ffaf8cbc30cf26b3726e26581c5f25ba99ab22a73c1c4942d0`，运行平台为 `linux/amd64`。
- Gateway 镜像为 `deerroom-app:v0.1.3`，运行环境中的 `DEER_NODE_VERSION` 为 `v0.1.3`，容器状态为运行且重启计数为 0。
- 迁移前后 `windows-media` 的 79 个可用视频 ID、媒体键、工作室、标题及权益计数逐条一致；当前为 79 个唯一媒体键、5 个工作室、2 条权益、0 个哈希标题。
- 本机回环健康、公网健康和公网首页均返回 `200`；Gateway 切换瞬间出现一次短暂 `502`，随后自动恢复并通过连续验收。
- 除 Gateway 外的 13 个小鹿及其他业务容器启动记录与部署前完全一致；宿主机 Caddyfile SHA-256 前后相同，未 reload Caddy。

### Notes
- `/opt/deer-screening-room/app.mp4-metadata-20260803-1`：本轮 104 发布目录，运行配置权限沿用部署前版本。
- `/opt/deer-screening-room/backups/mp4-metadata-before-20260803-1`：部署前环境、Compose、Gateway inspect、容器状态和 Caddyfile 哈希备份。
- `progress.md`：追加镜像发布、Gateway 部署、验收和回滚证据。
- 回滚方式：进入 `/opt/deer-screening-room/app.audit-20260803-1/deploy/cloud`，执行 `DEER_VERSION=project-audit-20260803-1 docker compose --project-name deer-screening-room-cloud --env-file .env -f compose.yaml -f compose.host-caddy.yaml up -d --no-build --no-deps gateway`；不使用 `-v`，不操作 Web、PostgreSQL、WireGuard、Caddy 或其他业务容器。

## 2026-08-03 - Task: 收敛严格媒体规范并清理旧节点部署兼容
### What was done
- 媒体节点收敛为只接收带 AAC 音频、非空 `title` 和合法 `deer_media_key` 的 AV1 MP4；删除 H.264、无音轨、其他容器格式和元数据缺失回退逻辑。
- 为目录缓存增加媒体规范版本，旧缓存升级后必须重新执行 `ffprobe`，验证通过后才恢复增量复用；不合规文件从目录跳过并记录到节点日志。
- 删除与网页生成安装包重复且无当前引用的 UGREEN Compose、registry 覆盖、UGREEN 环境示例和节点手工 WireGuard 示例；当前 Windows Compose 与网页生成的专属节点包继续保留。
- 同步媒体规范、架构、节点安装包说明与 NAS 部署文档；数据库迁移、API、权益模型、Range 播放和节点鉴权均未改变。

### Testing
- TDD 红灯：严格规则测试在旧实现下确认旧缓存未重新探测，缺元数据及旧格式文件仍被接受；节点包测试确认旧 README 仍声明元数据回退。
- TDD 绿灯：`go test ./internal/media ./internal/provisioning -count=1` 通过。
- `go test ./...`、`go test -race ./...`、`go vet ./...`：通过。
- 前端 Vitest 3 个文件/6 项通过，生产构建通过；Playwright 11 项通过。
- 根 Compose、云端 Compose、宿主 Caddy 组合和 Windows 节点 Compose 共 4 套配置解析通过；`git diff --check` 通过。
- 真实 Windows 媒体根目录探测 79 个 MP4，AV1/AAC、`title` 和 `deer_media_key` 不合规计数为 0。

### Notes
- `internal/media/scanner.go`、`internal/media/scanner_test.go`：实现并验证严格 AV1/AAC MP4 与必需元数据规则，旧缓存按规范版本重新探测。
- `internal/media/node.go`：媒体响应类型收敛为 `video/mp4`，删除其他容器 MIME 分支。
- `internal/provisioning/provisioning.go`、`internal/provisioning/provisioning_test.go`：节点包说明改为严格媒体规范并增加回归约束。
- `deploy/media-node/.env.ugreen.example`、`deploy/media-node/compose.registry.yaml`、`deploy/media-node/compose.ugreen.yaml`、`deploy/media-node/wg0.conf.example`：删除无当前引用的旧节点部署模板。
- `docs/architecture.md`、`docs/deployment-fnos.md`、`docs/media-library.md`：同步严格媒体规范与唯一节点包部署入口。
- `progress.md`：追加本轮实施、验证和回滚记录。
- 回滚方式：回退本轮代码提交即可恢复 v0.1.3 的格式兼容与旧节点部署模板；数据库、节点身份、权益和媒体文件均不需要恢复。本条记录阶段尚未重建本机节点或 104 服务。

## 2026-08-03 - Task: 清理本机与 104 历史小鹿运行残留
### What was done
- 本机 Docker 镜像只保留当前运行的 `deerroom-app:v0.1.3` 和 `ghcr.io/lwy183178053/deer-screening-room:v0.1.2` 回滚镜像；删除未被容器引用的旧小鹿开发、节点 provisioning、媒体视图和旧版本镜像标签。
- 按已确认策略删除仓库外双样本目录和 79 个旧哈希媒体文件完整备份，释放约 38 GiB；当前媒体根目录和 79 个迁移后 MP4 保留。
- 104 仅保留当前发布目录 `app.mp4-metadata-20260803-1` 与回滚目录 `app.audit-20260803-1`，删除其他历史发布目录、旧备份集合和约 306 MiB 发布包。
- 104 删除未被容器引用的旧小鹿镜像；保留当前运行所需镜像、`v0.1.3` 当前镜像和 `project-audit-20260803-1` Gateway 回滚镜像。未重建、停止或重启任何容器，未 reload Caddy。

### Testing
- 本机清理后 `deer-node` 继续使用 `deerroom-app:v0.1.3` 运行，`deer-wg` 保持健康；当前媒体根目录仍为 79 个 MP4，两份迁移目录均已不存在。
- 本机历史小鹿镜像清理后仅保留当前 `v0.1.3` 和 `v0.1.2` 回滚镜像；ModelRoute、Redis、MySQL 等其他容器未被清理命令操作。
- 104 当前目录与回滚目录的 Compose 均可完整解析；当前目录保留权限为 `600` 的运行环境文件和 WireGuard 配置。
- 104 清理前后全部运行容器的镜像、启动时间和重启次数逐项一致；Caddyfile SHA-256 均为 `4d598e995f68224c2a717e03c853cbab544249904cbdb73b912efc6229e921a6`。
- 104 回环与公网 `/api/v1/health` 清理后均返回 `200`；ModelRoute、Sub2API 及其数据库和 Redis 状态保持原样。

### Notes
- `progress.md`：追加本机镜像、媒体迁移备份和 104 历史发布残留的清理证据。
- 仓库文件没有因运行残留清理发生其他变化；本机 Docker 卷、当前节点配置、源媒体和 104 数据库卷均保留。
- 本机媒体迁移备份与双样本已按用户确认永久删除，不能从本机回滚；当前 79 个迁移后文件和 NAS 副本是现行媒体来源。
- 代码回滚点为提交 `dc3116a` 的父提交 `79a3fbb`；本机节点运行回滚可重新拉取固定标签 `v0.1.2`。104 Gateway 回滚使用 `app.audit-20260803-1` 与镜像 `deerroom-app:project-audit-20260803-1`，只重建 Gateway 且不使用 `-v`。

## 2026-08-03 - Task: 准备发布 v0.1.4 严格媒体节点镜像
### What was done
- 将 Gateway 生成节点安装包的默认版本、根 Compose、云端 Compose 和媒体节点环境示例统一更新为 `v0.1.4`。
- 保持镜像仓库、节点身份、WireGuard、数据库、权益和 Range 播放接口不变；本轮版本提交只负责让后续节点包与部署固定使用严格媒体规范镜像。
- 更新 NAS 部署文档，明确新下载的节点安装包固定使用 `v0.1.4`。

### Testing
- TDD 红灯：配置与节点包测试在旧默认值 `v0.1.3` 下按预期失败；切换默认值后定向测试通过。
- `go test ./...`、`go test -race ./...`、`go vet ./...`：通过。
- 前端 Vitest 3 个文件/6 项、生产构建和 Playwright 11 项：通过。
- 根 Compose、云端 Compose、宿主 Caddy 组合和 Windows 节点 Compose 共 4 套配置解析通过；`git diff --check` 通过。

### Notes
- `compose.yaml`、`deploy/cloud/.env.example`、`deploy/cloud/compose.yaml`、`deploy/media-node/.env.example`：默认节点版本更新为 `v0.1.4`。
- `internal/config/config.go`、`internal/config/config_test.go`：Gateway 默认节点版本及回归测试更新为 `v0.1.4`。
- `internal/provisioning/provisioning.go`、`internal/provisioning/provisioning_test.go`：节点包缺省版本及回归测试更新为 `v0.1.4`。
- `docs/deployment-fnos.md`：同步 NAS 节点安装包固定版本。
- `progress.md`：追加本轮发布准备、验证和回滚记录。
- 回滚方式：回退本轮发布提交即可恢复 `v0.1.3` 默认版本；尚未切换运行容器时不需要操作节点、Gateway、数据库或媒体文件。

## 2026-08-03 - Task: 发布 v0.1.4 并部署 Windows 节点与 104 Gateway
### What was done
- 推送 `v0.1.4` 标签并通过 GitHub Actions 发布 GHCR 镜像，同时更新 `latest`；NAS 节点未远程操作。
- 当前 Windows 节点保留原节点身份、媒体目录、封面缓存和 WireGuard，仅将 `deer-node` 更新为 `v0.1.4`；`deer-wg` 未重建。
- 在 104 创建独立发布目录 `/opt/deer-screening-room/app.v0.1.4-20260803-1`，复用权限受限的运行配置与 WireGuard 配置，仅重建 Gateway，使新下载节点包固定使用 `v0.1.4`。
- 本机与 104 的回滚资源统一收敛为 `v0.1.3`；删除本机 `v0.1.2` 镜像和 104 的旧 `project-audit` 回滚目录及镜像。

### Testing
- GHCR `v0.1.4` 与 `latest` 的 OCI 摘要均为 `sha256:ceeaa34ef17f9a41c81d015cf2309e21f0365d3974b4c17f4df87c2a5247f8d9`，`linux/amd64` manifest 摘要均为 `sha256:23082a3f22348176c426081a98e58b1d7a61831d40f658a60b745ff2bca45fcc`。
- Windows `deer-node` 运行 `v0.1.4` 且重启次数为 0；节点健康接口成功，WireGuard 有最近握手，`deer-wg` 启动时间与更新前一致。
- Windows 目录缓存为 79 条，79 条均为严格规范版本、`ready`、AV1/AAC；真实媒体 Range 请求返回 `206 Partial Content`、1024 字节和 `Content-Range: bytes 0-1023/558673160`。
- 104 Gateway 运行 `v0.1.4` 且重启次数为 0，运行环境中的 `DEER_NODE_VERSION` 为 `v0.1.4`；Windows 与 NAS 两个节点均在线且最近 90 秒内有心跳。
- 生产目录当前为 Windows 79 个视频/5 个工作室、NAS 21 个视频/4 个工作室，共 100 个可用且 `ready` 的视频，节点不合规视频计数为 0；原有 9 条权益记录保留。
- 104 回环健康、公网健康和公网首页均返回 `200`；Web、PostgreSQL、WireGuard、WireGuard provisioner、ModelRoute、Sub2API 及其数据库和 Redis 的启动时间与重启次数未变化。
- Caddyfile SHA-256 仍为 `4d598e995f68224c2a717e03c853cbab544249904cbdb73b912efc6229e921a6`，本轮未 reload Caddy。

### Notes
- `progress.md`：追加 `v0.1.4` 镜像发布、Windows 节点和 104 Gateway 部署证据。
- `/opt/deer-screening-room/app.v0.1.4-20260803-1`：104 当前发布目录；`/opt/deer-screening-room/app.mp4-metadata-20260803-1`：唯一保留的 `v0.1.3` 回滚目录。
- 本机忽略的 `deploy/media-node/.env` 已固定 `v0.1.4`；NAS 保留现有节点身份和配置，由用户在面板将版本改为 `v0.1.4` 后拉取重建。
- 回滚方式：Windows 将 `DEER_VERSION` 攁回 `v0.1.3` 并仅重建 `node`；104 进入 `app.mp4-metadata-20260803-1/deploy/cloud`，使用该目录的 `.env` 和两份 Compose 文件执行 `up -d --no-build --no-deps gateway`，不使用 `-v`，不操作其他服务。

## 2026-08-04 - Task: 执行新媒体解压后的 AV1/AAC 转码
### What was done
- 修复 Windows 转码脚本在 media-node 未运行时对空 Compose 输出调用 `.Trim()` 的问题，并让维护入口在停止失败时清理刚创建的锁。
- 继续执行 `-Mode New`，处理新解压媒体；已完成编码、完整解码校验、原子替换和元数据整理。

### Testing
- `半岛2024` 目录共 65 个 MP4，ffprobe 验收 65/65：AV1、最长边不超过 1280、AAC 双声道 48 kHz、约 128 kbps，且 `title`/`deer_media_key` 元数据完整。
- 维护锁不存在，`.av1.tmp.mp4` 和 `.av1-original.tmp` 临时/备份文件数量均为 0。
- 转码进程正常退出；本轮未启动原本已停止的 media-node，也未操作其他容器。

### Notes
- `scripts/transcode-av1-720p.ps1`：处理 Compose 无容器输出并在维护入口失败时清理锁。
- `docs/local-archive-extraction.md`：补充节点未运行时的转码行为和锁清理说明。
- `progress.md`：记录本轮转码与验收证据。
- 回滚方式：回退本轮脚本/文档提交即可；媒体替换已完成且无临时备份，需使用媒体外部备份恢复原片时再单独执行恢复流程。

## 2026-08-04 - Task: 兑换说明链接点击与移动端适配
### What was done
- 将兑换说明中的 HTTP/HTTPS 地址安全拆分为可点击链接，新标签页打开并保留普通文字、换行和标点；没有使用 HTML 注入渲染。
- 为兑换说明及链接增加长单词断行规则，保证 320px、390px 和 430px 手机宽度下不产生横向溢出。
- 保持兑换说明 API、数据库、兑换逻辑及后台编辑方式不变。

### Testing
- TDD 红灯确认原实现缺少链接分段模块；新增实现后链接分段测试 3 项通过。
- 前端 Vitest 4 个文件/9 项通过，生产构建通过。
- Playwright 13 项通过；桌面验证链接的 `href`、`target` 和 `rel`，320px、390px、430px 验证长链接边界、账户控件和页面无横向溢出。
- 人工检查三张移动端截图，链接均在兑换说明块内完整换行。

### Notes
- `web/src/redeemNotice.ts`、`web/src/redeemNotice.test.ts`：增加仅识别 HTTP/HTTPS 的安全文本分段逻辑和边界测试。
- `web/src/App.vue`：将兑换说明按普通文本和外部链接渲染。
- `web/src/styles.css`：增加兑换说明长链接断行及链接颜色。
- `web/e2e/app.spec.ts`：增加桌面链接属性和三种手机宽度布局验收。
- `docs/credits.md`：记录兑换说明链接的新标签页及移动端断行行为。
- `progress.md`：追加本轮实现、测试和回滚记录。
- 回滚方式：回退本轮前端提交并仅重建 Web 服务；不需要操作 Gateway、数据库、WireGuard、节点或媒体文件。

## 2026-08-04 - Task: 部署兑换说明链接前端修复
### What was done
- 在 104 创建独立发布目录 `app.web-links-20260804-1`，保留原 Web 容器检查点和旧 Web 镜像。
- 只构建并重建 `deer-screening-room-cloud-web-1`；Gateway、PostgreSQL、WireGuard、WireGuard provisioner、Caddy 和其他业务容器未重启或修改。
- 将生产 Web 更新为包含兑换说明可点击链接和移动端断行的前端构建。

### Testing
- 104 Web Compose 配置校验和 `build web` 通过，镜像摘要为 `sha256:c7e26cfd144a1cf84436672bc25c6e988a3ab87546f6a8db6ff3d3a254bd6c8f`。
- 回环首页、回环 `/api/v1/health` 和 `https://xiaolu.lwylink.xyz/` 均返回 `200`。
- 真实域名登录后，390px 手机视口验证购买链接的 HTTPS 协议、`target="_blank"`、`rel="noopener noreferrer"` 和无横向溢出。
- Web 重建后重启次数为 0；Gateway、PostgreSQL、WireGuard、provisioner 启动时间和重启次数未变化；宿主 Caddyfile SHA-256 仍为 `4d598e995f68224c2a717e03c853cbab544249904cbdb73b912efc6229e921a6`。

### Notes
- `/opt/deer-screening-room/app.web-links-20260804-1`：104 本轮 Web 发布目录。
- `/opt/deer-screening-room/app.web-links-20260804-1/deploy/cloud/web-before.json`：部署前 Web 容器检查点。
- `progress.md`：追加生产部署、镜像摘要和验收证据。
- 回滚方式：在新发布目录执行 `DEER_VERSION=node-counts-20260803-1 docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml up -d --no-build --no-deps web`；不使用 `-v`，不操作其他服务。

## 2026-08-04 - Task: 兑换码分页与管理员移动端适配
### What was done
- 将管理员兑换码列表从“加载更多”改为 50 条/页的明确分页，切换页时替换当前列表；状态筛选、搜索和创建后均回到第 1 页。
- 将兑换码创建表单和兑换说明编辑器在手机端改为紧凑单列，保留兑换码表格局部横向滚动并限制整页宽度。
- 未修改兑换码 API、数据库、兑换规则或其他管理页面。

### Testing
- TDD 红灯：分页控件和移动端验收在旧实现下失败；完成实现后定向 Playwright 4 项通过。
- Vitest 4 个文件/9 项通过；生产构建通过。
- Playwright 全部 17 项通过：73 条兑换码分页为 50+23，上一页/下一页、筛选和搜索回到第 1 页；单页状态隐藏分页。
- 320px、390px、430px 管理员兑换码页面验证创建区和说明区上下堆叠、按钮/文本框可见、分页可操作且整页无横向溢出。
- `git diff --check` 通过。

### Notes
- `web/src/App.vue`：加入兑换码分页状态、页数计算、翻页和单页替换逻辑。
- `web/src/styles.css`：增加分页样式、移动端表单收紧和表格溢出边界。
- `web/e2e/app.spec.ts`：增加分页、筛选/搜索重置和 320/390/430 移动端验收。
- `progress.md`：记录本轮实现与测试证据。
- 回滚方式：回退本轮前端提交并仅重建 Web 服务；不操作 Gateway、数据库、WireGuard、Caddy、节点或媒体文件。

## 2026-08-04 - Task: 部署兑换码分页与管理员移动端适配
### What was done
- 在 104 创建独立发布目录 `app.redeem-pagination-20260804-1`，并将部署前 Web 镜像固定为 `deerroom-web:redeem-links-20260804-1`。
- 只构建并重建 `deer-screening-room-cloud-web-1`，上线兑换码分页和管理员移动端紧凑布局。
- Gateway、PostgreSQL、WireGuard、WireGuard provisioner、Caddy、媒体节点和其他业务容器未重启或修改。

### Testing
- 104 Web Compose 配置校验和 `build web` 通过，新 Web 镜像摘要为 `sha256:402596e5762410df8dffe8dbc90e8052662deccbe269c59566ac3a5fb34bafd4`。
- 回环首页、回环 `/api/v1/health` 和 `https://xiaolu.lwylink.xyz/` 均返回 `200`，线上静态资源为 `assets/index-DhrR5OqJ.js`。
- 真实管理员页面当前共 74 条兑换码：第一页 50 条、第二页 24 条；第 2 页 API 和最终 DOM 行数一致。
- 320px、390px、430px 真实域名验收均通过：创建区与说明区上下堆叠、分页可见可操作、整页无横向溢出。
- Web 重建后重启次数为 0；Gateway、PostgreSQL、WireGuard、provisioner 的启动时间和重启次数未变化；宿主 Caddyfile SHA-256 仍为 `4d598e995f68224c2a717e03c853cbab544249904cbdb73b912efc6229e921a6`。

### Notes
- `/opt/deer-screening-room/app.redeem-pagination-20260804-1`：104 本轮 Web 发布目录。
- `/opt/deer-screening-room/app.redeem-pagination-20260804-1/deploy/cloud/web-before.json`：部署前 Web 容器检查点。
- `progress.md`：追加生产部署、分页实测和服务隔离证据。
- 回滚方式：在本轮发布目录执行 `DEER_VERSION=redeem-links-20260804-1 docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml up -d --no-build --no-deps web`；不使用 `-v`，不操作其他服务。

## 2026-08-05 - Task: 处理学姐学妹工作室新增媒体
### What was done
- 仅处理 `E:\BaiduNetdiskDownload\学姐学妹`：22 个 7z 全部测试、解压成功后删除，得到并处理 21 个 MP4；其他工作室未执行解压或转码。
- 将 21 个视频转换为最长边不超过 1280 的 AV1 MP4，音频保持 AAC-LC、48 kHz、双声道和名义 128 kbps，并写入显示标题与稳定媒体键。
- 修正转码验收对 AAC 平均码率的误判：编码目标仍为 128 kbps，实测验收范围调整为 96-136 kbps，以兼容静音或低复杂度音频。
- 本机 `deer-node` 原本未运行，本轮保持原状态；处理完成后清除了本轮临时日志。

### Testing
- 实际批次输出 `Encoding 21/21`、`AV1 conversion completed: 21 file(s).`，错误日志为空。
- PowerShell 语法检查与元数据脚本测试通过；`New` 模式复扫结果为 0 个待转换、0 个待更新元数据。
- 对 21 个 MP4 全量执行 ffprobe：AV1、最长边不超过 1280、AAC-LC 双声道 48 kHz、音频平均码率 109.7-128.9 kbps、标题元数据和 43 字符媒体键全部通过。
- 21 个媒体键内部唯一，并与媒体根目录其余 55 个 MP4 的媒体键只读比对无冲突。
- 目标目录内压缩包、转码临时文件、原片临时备份、维护锁和处理日志均为 0。

### Notes
- `scripts/transcode-av1-720p.ps1`：将 AAC 实测平均码率验收范围调整为 96-136 kbps，保留 128 kbps 编码参数。
- `docs/local-archive-extraction.md`：说明低复杂度音频的实测平均码率可能低于名义 128 kbps。
- `E:\BaiduNetdiskDownload\学姐学妹\*.mp4`：21 个新增媒体已完成 AV1 720p 转换和元数据写入。
- `progress.md`：记录本轮单工作室处理、验证证据和回滚点。
- 回滚方式：代码与文档可回退本轮提交；媒体已在逐文件完整解码校验后替换，目录内未保留 1080p 原片，媒体回滚需从外部备份恢复或重新下载源文件。

## 2026-08-05 - Task: 工作室分类自适应折叠与紧凑排列
### What was done
- 将首页工作室分类从单行横向滚动改为桌面和手机默认两行的响应式布局，实际溢出时提供“显示更多/收起”。
- 保持“推荐”第一位，以接口原顺序作为每行优先项，并按按钮真实宽度使用后续短分类回填空位；相同数据和屏宽下排列稳定。
- 选择工作室后保持展开状态；超长名称在按钮内省略，分类区和页面均不产生横向溢出。
- 未修改工作室 API、视频筛选、分页、数据库、节点或播放链路。

### Testing
- TDD 红灯确认旧页面没有展开控件；完成实现后排列算法 5 项定向测试和分类区 Playwright 5 项通过。
- Vitest 5 个文件/14 项通过，前端生产构建通过。
- Playwright 全部 21 项通过，覆盖两行折叠、展开/收起、选择后保持展开、工作室较少时隐藏控件，以及 320px、390px、430px 手机布局。
- 人工检查桌面、320px 和 430px 截图：分类紧凑回填、长名称省略、控制按钮位置及页面宽度符合设计。

### Notes
- `web/src/studioLayout.ts`、`web/src/studioLayout.test.ts`：增加确定性宽度装箱算法及单元测试。
- `web/src/App.vue`：增加分类测量、响应式重排、溢出检测和展开状态。
- `web/src/styles.css`：增加两行折叠、长名称省略及桌面/手机控制按钮样式。
- `web/e2e/app.spec.ts`：增加多工作室桌面和三种手机宽度验收。
- `docs/media-library.md`：记录工作室分类的新排列和折叠行为。
- `progress.md`：记录本轮实现、验证和回滚点。
- 回滚方式：回退本轮前端提交并仅重建 Web 服务；不操作 Gateway、数据库、WireGuard、Caddy、节点或媒体文件。

## 2026-08-05 - Task: 恢复鲁R媒体批次并修正 AAC 验收边界
### What was done
- 诊断上一批在第 40/87 个视频停止的原因：输出为合法 AV1 720p、AAC-LC 双声道 48 kHz，音频实测 95.186 kbps，仅因旧的 96 kbps 下限误判；原片没有被替换。
- 将转码验收下限调整为 80 kbps，编码目标仍保持名义 AAC 128 kbps；删除唯一失败临时输出后重新启动当前目录完整压缩包批次。
- 当前批次只锁定 `E:\BaiduNetdiskDownload\鲁R（虐恋天使）`，不处理其他工作室；下载中的文件不混入批次。

### Testing
- 失败输出和原片均用 ffprobe 检查，失败输出轨道、尺寸、元数据和完整性均有效。
- 重启后 54 个完整压缩包全部测试、解压并删除成功，后台日志已进入 `Encoding 1/102`；最终 102 个视频的完整转码验收仍由后台任务继续执行。

### Notes
- `scripts/transcode-av1-720p.ps1`：将 AAC 实测验收范围下限调整为 80 kbps。
- `docs/local-archive-extraction.md`：同步说明 80-136 kbps 实测验收范围和名义 128 kbps 目标。
- `E:\BaiduNetdiskDownload\鲁R（虐恋天使）\.deer-process.*.log`：记录本批次后台解压和转码输出。
- 回滚方式：后台任务失败会保留当前原片和失败临时输出；停止后台 PowerShell 后删除临时输出即可，已成功替换的视频需从外部备份或重新下载恢复。

## 2026-08-05 - Task: 工作室分类“更多”按钮与手机标签收紧
### What was done
- 将折叠按钮文字改为“更多”，并与折叠区域第二行标签同行显示；展开后的“收起”作为分类列表最后一个按钮。
- 手机端分类标签高度调整为 36px、缩小内边距和间距，保留两行折叠、长名称省略和无横向溢出。
- 补强异步布局提交后的溢出复测，覆盖真实规模的 6 个工作室数据组合；不修改 API、筛选、分页、节点或播放逻辑。

### Testing
- Vitest 5 个文件/14 项通过，生产构建通过。
- Playwright 全部 22 项通过，覆盖“更多”同行、展开/收起、选择后保持展开，以及 320px、390px、430px 手机布局。
- `git diff --check` 通过；人工检查桌面和手机截图，按钮高度、位置、长名称省略和页面宽度符合要求。

### Notes
- `web/src/App.vue`：调整“更多/收起”渲染位置并补强异步溢出检测。
- `web/src/styles.css`：将控制按钮改为分类标签样式并收紧手机标签尺寸。
- `web/e2e/app.spec.ts`：更新按钮文案、同行尺寸断言和 6 工作室异步回归用例。
- `docs/media-library.md`：记录“更多/收起”同行分类行为。
- `progress.md`：记录本轮 UI 细化、验证和部署回滚点。
- 回滚方式：回退本轮前端提交并只重建 Web 服务；不操作 Gateway、数据库、WireGuard、Caddy、节点或媒体文件。

## 2026-08-05 - Task: 部署工作室分类“更多”按钮与手机标签收紧
### What was done
- 在 104 创建独立发布目录 `/opt/deer-screening-room/app.studio-filter-20260805-2`，固定部署前 Web 镜像并只重建 `deer-screening-room-cloud-web-1`。
- 上线“更多”同行按钮、手机 36px 分类标签和异步溢出检测；没有重启或修改 Gateway、PostgreSQL、WireGuard、provisioner、Caddy、节点及其他业务容器。

### Testing
- Web Compose 配置校验和镜像构建通过；新镜像摘要为 `sha256:47603661a044fa2a5b6ffdb236f13efa41709ab4b2ce028250148eca2e60ef73`。
- 回环首页、回环 `/api/v1/health` 和 `https://xiaolu.lwylink.xyz/` 均返回 `200`；真实静态资源为 `assets/index-DjCgi5Nb.js` 与 `assets/index-DLkBI62-.css`。
- 真实域名浏览器验证：1440px 下 6 个工作室一行且不显示控制按钮；320/390/430px 下“更多”与第二行同行，展开后“收起”可见，三种宽度均无横向溢出。
- Web 新容器重启次数为 0；Gateway、PostgreSQL、WireGuard、provisioner 的启动时间、镜像和重启次数与部署前一致；宿主 Caddyfile SHA-256 仍为 `4d598e995f68224c2a717e03c853cbab544249904cbdb73b912efc6229e921a6`。

### Notes
- `/opt/deer-screening-room/app.studio-filter-20260805-2`：本轮 Web 发布目录。
- `/opt/deer-screening-room/app.studio-filter-20260805-2/deploy/cloud/web-before.json`：部署前 Web 容器检查点。
- `progress.md`：记录生产镜像、真实域名布局和服务隔离证据。
- 回滚方式：在本轮发布目录使用 `DEER_VERSION=studio-filter-before-20260805-2 docker compose --env-file .env -f compose.yaml -f compose.host-caddy.yaml up -d --no-build --no-deps web`；不使用 `-v`，不操作其他服务。

## 2026-08-05 - Task: 工作室筛选弹窗与品牌图标正式同步
### What was done
- 将首页工作室分类改为标题右侧的“工作室”按钮；桌面端使用居中弹窗，手机端使用底部抽屉，选择后关闭并沿用现有 `studio_id` 筛选。
- 删除旧标签墙的折叠、更多/收起、ResizeObserver 重排状态；新增按实际标签宽度进行全局行装箱、每行居中的布局，并保留超宽名称的容器约束。
- 使用提供的小鹿 SVG 更新品牌图标、单色资源、favicon 和 PWA 图标；品牌副标题改为“小鹿怡情”。
- 更新前端单元测试、Playwright 选择器和媒体库文档，清理旧标签墙测试与无用数据。

### Testing
- `web`: `npm.cmd test`，5 个测试文件、13 项通过。
- `web`: `npm.cmd run build`，Vue 类型检查和生产构建通过。
- `web`: `npm.cmd run test:e2e`，21 项通过；覆盖桌面目录、弹窗选择、标题切换、兑换码/管理/播放流程及 320px、390px、430px 移动端无横向溢出。
- `git diff --check` 通过；未修改后端、数据库、节点或媒体文件。

### Notes
- `web/src/App.vue`：替换工作室标签墙为弹窗/底部抽屉，并接入标题同步和筛选关闭逻辑。
- `web/src/styles.css`：增加标题右侧按钮、弹窗标签行和移动端样式，删除旧折叠标签墙样式。
- `web/src/studioLayout.ts`：实现带时间预算的全局行装箱算法，避免大量工作室阻塞浏览器。
- `web/src/studioLayout.test.ts`：覆盖推荐首位、最少行数、稳定性和超宽标签。
- `web/src/App.test.ts`：覆盖弹窗入口和标签渲染。
- `web/e2e/app.spec.ts`：将旧标签墙用例改为弹窗、选中关闭和三种手机宽度用例。
- `web/src/components/BrandLogo.vue`：品牌副标题改为“小鹿怡情”。
- `web/src/assets/deer-mark.svg`、`web/src/assets/deer-mark-mono.svg`、`web/public/favicon.svg`、`web/public/icon-192.png`、`web/public/icon-512.png`：替换为用户提供的小鹿图标资源。
- `docs/media-library.md`：记录新的工作室弹窗筛选行为。
- `progress.md`：记录本轮实施、验证和回滚点。
- 回滚方式：回退本轮前端提交，并仅使用旧 Web 镜像执行 `docker compose up -d --no-build --no-deps web`；不操作 Gateway、数据库、WireGuard、Caddy、节点或媒体文件。
