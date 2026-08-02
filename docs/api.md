# HTTP API

公开读取：`GET /api/v1/studios`、`GET /api/v1/videos`、`GET /api/v1/videos/{id}` 和视频封面。

视频目录和已购库使用 `page` 查询参数分页，每页最多 20 条，响应统一为 `{ "videos": [], "page": 1, "page_size": 20, "total": 0 }`；首页标题为“推荐”，每次进入首页生成新的随机 `seed`，前端通过右下角分页按钮切换页面。输入搜索词时隐藏工作室分类条，仅显示匹配视频。

公开目录支持 `GET /api/v1/videos?seed=<positive-integer>&page=<page>`。相同 `seed` 在各页保持稳定随机顺序，不同种子产生新的随机顺序；非法种子返回 `422 invalid_seed`。首页仅在未搜索、未选择工作室时携带种子，搜索与工作室筛选保持原有更新时间排序。

账户：注册、登录、当前账户和退出位于 `/api/v1/auth/*`。`GET /api/v1/auth/captcha?purpose=register|login` 返回一次性 PNG 图片挑战；注册始终提交 `captcha_id` 和 `captcha_answer`，登录首次失败后要求验证码。鹿币与永久视频权益包括 `/api/v1/wallet`、`/wallet/redeem` 和 `/videos/{id}/unlock`；每部视频统一 1 鹿币。`GET /api/v1/commerce` 返回单片价格和兑换说明。

播放：前端 `/watch/{videoId}` 使用自托管 Artplayer 独立播放页；`POST /api/v1/videos/{id}/playback` 创建单账号唯一会话，返回同源 `/api/v1/playback/{id}/stream`。流接口支持 GET、HEAD、Range 和缓存条件头，播放器不加载广告、第三方脚本或外部链接。

管理员：`users`、`redeem-codes`、`redeem-notice`、`nodes` 和播放速率 `settings`。兑换码创建响应为一次性 UTF-8 TXT 下载（每行一个纯兑换码）；`GET/PUT /api/v1/admin/redeem-notice` 用于读取和保存充值区兑换说明；`GET /api/v1/admin/redeem-codes` 支持 `status`、`q` 和 `page`，返回单码状态、使用者和统计数量。`GET/PUT /api/v1/admin/settings` 只读取和保存 `user_stream_mbps`。节点响应包含 `scan_status`、`last_scan_at` 和可选 `scan_error`；重新扫描立即返回 `202`。用户查询支持 `q` 和 `page`，并返回 `all_total`；`POST /api/v1/admin/users/{id}/credits` 使用 `delta`、可选 `reason` 和幂等 `request_id` 调整鹿币。

视频管理不提供人工编辑接口。标题、工作室和封面由 NAS 目录扫描产生；管理员只能在节点接口触发重新扫描。

节点内部接口：`POST /api/v1/internal/media/sync` 和 `heartbeat` 使用与节点名称、固定内部地址绑定的 Bearer API Token。媒体节点内部读取使用该节点独立的 `X-Relay-Token`，不经过公网 Caddy。目录版本未变化时节点只发送心跳。

错误响应统一为 `{ "error": { "code": "...", "message": "..." } }`。
