# Security

- 密码使用 Argon2id；会话 Cookie 为 HttpOnly、SameSite=Lax，生产环境启用 Secure。数据库只保存会话令牌 SHA-256。
- 注册始终使用一次性图片验证码；登录失败后进入验证码模式。验证码和频率窗口只保存在 Gateway 内存，Argon2 同时计算数量默认限制为 4。
- 所有登录态变更接口要求会话绑定的 `X-CSRF-Token`。管理员接口同时检查角色，停用用户会删除其会话。
- Caddy 在公网拒绝 `/api/v1/internal/*`。每个媒体节点由 `DEER_NODE_CREDENTIALS` 绑定唯一名称、WireGuard 地址和 API Token；Relay Token 与旧 HTTP 视频端点已删除。
- 浏览器只提交 SDP，Gateway 丢弃客户端 ICE 配置，并向节点注入服务端临时 TURN 配置和管理员策略，防止客户端指定任意 TURN 服务或绕过 relay-only 模式。
- “允许节点直连”默认关闭。浏览器和节点都强制 relay 时，观看者只看到 104 的 TURN relay candidate，媒体节点也只看到 TURN 对端；coturn 主机仍能看到双方连接来源。开启直连后，WebRTC 对端可能看到节点公网候选，mDNS 只负责隐藏局域网 host 地址。
- STUN 地址只在管理员开启“允许节点直连”时下发；关闭时不暴露 STUN 候选，浏览器和节点均保持 relay-only。直连模式不承诺隐藏节点公网位置，隐私要求必须使用 TURN-only。
- coturn 使用短期 HMAC 凭据，生产 relay 禁止回环、链路本地和 RFC1918 私网目标。TURN 共享密钥只放在权限受限且被 Git 忽略的运行配置中。
- 视频目录只读挂载。浏览器和 Gateway 不接收节点真实路径；媒体键只在节点内存映射为文件。节点内部错误不回传文件路径、FFmpeg 参数、Token 或 WireGuard 地址。
- Gateway 不转发视频字节，不执行用户带宽限速；媒体节点不设置应用级连接槽位。账号仍保留每分钟播放会话/信令请求频率保护，防止持续创建会话耗尽进程资源。
- WebRTC DTLS-SRTP 保护媒体传输，但网页 DRM 不在范围内，观看者仍可能保存自己已经接收和解码的内容。
- 鹿币只能通过兑换码或管理员调账获得，只能站内消费，不提供转账、提现或用户间交易。管理员调账要求幂等请求编号，并与钱包流水和审计记录在同一事务提交。
