# Progress

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
