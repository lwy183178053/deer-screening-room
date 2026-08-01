# HTTP API

公开读取：`GET /api/v1/studios`、`GET /api/v1/videos`、`GET /api/v1/videos/{id}` 和视频封面。

视频目录和已购库使用 `page` 查询参数分页，每页 20 条，响应统一为 `{ "videos": [], "page": 1, "page_size": 20, "total": 0 }`。公开目录支持正整数 `seed` 保持稳定随机顺序，也支持 `q` 和 `studio_id`；非法种子返回 `422 invalid_seed`。

账户：注册、登录、当前账户和退出位于 `/api/v1/auth/*`。`GET /api/v1/auth/captcha?purpose=register|login` 返回一次性图片验证码。鹿币与永久权益包括 `/api/v1/wallet`、`/wallet/redeem` 和 `/videos/{id}/unlock`；`GET /api/v1/commerce` 返回单片价格和兑换说明。

## WebRTC 播放

- `POST /api/v1/videos/{id}/p2p/session`：校验登录、CSRF、视频在线状态和永久权益，创建六小时播放会话，返回 `session_id`、`expires_at`、`p2p_enabled` 与临时 `ice_servers`。
- `POST /api/v1/p2p/{session_id}/offer`：只接受浏览器 `sdp` 和 `type`。Gateway 忽略客户端提供的 ICE 配置，并使用服务端 TURN 配置和管理员直连策略向节点转发 offer，响应节点 answer。
- `DELETE /api/v1/p2p/{session_id}`：撤销播放会话并通知节点关闭 PeerConnection、FFmpeg 和 RTP 管线。重复关闭返回 `204`。
- `POST /api/v1/videos/{id}/playback` 与 `GET/HEAD /api/v1/playback/{id}/stream` 已删除并返回 `404`。

关闭“允许节点直连”时，浏览器和节点都使用 `relay` ICE 策略；开启后使用 `all` 并由 candidate pair 决定直连或 TURN。Gateway 只承载鉴权、信令和会话状态，不转发视频字节。

## 管理员与节点

管理员接口包括 `users`、`redeem-codes`、`redeem-notice`、`nodes` 和 `p2p-settings`。`GET/PUT /api/v1/admin/p2p-settings` 读取和保存 `{ "p2p_enabled": false }`。用户查询支持 `q` 和 `page`；`POST /api/v1/admin/users/{id}/credits` 使用 `delta`、可选 `reason` 和幂等 `request_id` 调整鹿币。

节点同步与心跳使用 `POST /api/v1/internal/media/sync` 和 `heartbeat`。Gateway 到节点使用 `POST /internal/webrtc/offer`、`POST /internal/webrtc/close`、封面读取和重新扫描；全部使用与节点名称和固定 WireGuard 地址绑定的 Bearer API Token。媒体节点不再提供 HTTP 视频读取端点。

错误响应统一为 `{ "error": { "code": "...", "message": "..." } }`；节点内部错误只记录在节点日志，公网响应不包含文件路径、FFmpeg 参数或内部地址。
