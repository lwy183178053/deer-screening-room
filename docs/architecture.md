# Architecture

小鹿放映室由 Vue 静态站点、Go Gateway、PostgreSQL、云端 coturn 和一个或多个媒体节点组成。生产环境的 Gateway 与媒体节点使用 WireGuard 作为目录同步、心跳、封面和 WebRTC 信令控制通道。

浏览器只通过 HTTPS 域名访问 Web 与 Gateway。已有宿主机 Caddy 的服务器使用 `compose.host-caddy.yaml`，Web 容器只绑定 `127.0.0.1:28200`；内部 Caddy 拒绝所有公网 `/api/v1/internal/*` 请求。coturn 独立监听 `3478/tcp`、`3478/udp` 和 `49160-65535/udp`，不占用 `80/443`。

播放数据流如下：

1. 浏览器向 Gateway 创建播放会话并取得临时 TURN REST 凭据。
2. 浏览器生成 SDP offer，Gateway 根据已授权会话查找媒体节点。
3. Gateway 通过 WireGuard 将 offer、服务端 ICE 配置和当前直连策略转发给节点。
4. 节点用 FFmpeg 读取只读媒体文件。浏览器 offer 已声明支持的 H.264 profile 直接复用为 RTP；未协商的 High 等 profile 在会话内用低延迟 Baseline 转码，AAC 转 Opus；Pion 返回 SDP answer。
5. 媒体字节在浏览器与节点间直连，或经 coturn 中继；Gateway 不接收媒体字节。

管理员设置 `p2p_enabled=false` 为默认隐私模式，浏览器和节点两端均强制 `ICETransportPolicyRelay`，观看者只看到 TURN relay candidate。设置为 `true` 时允许 host/server-reflexive candidate；Pion 使用 mDNS 隐藏局域网 host 地址，但直连仍可能向 WebRTC 对端公开公网候选，这是低延迟与节点位置隐私之间的明确取舍。

媒体节点保存真实路径及媒体键映射，Gateway 只保存媒体键、工作室、标题和技术元数据。一级目录对应工作室；视频消失时标记不可用，不删除永久权益。节点离线超过 90 秒后，其内容从公开目录隐藏。

播放会话有效期六小时。创建新会话会在用户行锁保护下撤销同账号旧会话；创建与信令接口继续按账号执行每分钟请求频率保护。节点 PeerConnection 断开、会话关闭、FFmpeg 结束或 TTL 到期时都会清理 FFmpeg 与 RTP 资源，不设置应用级带宽或连接数量上限。

每个节点只使用一份独立 API Token，Gateway 同时绑定节点名称、Token 和 WireGuard `base_url`。节点之间没有媒体副本或自动故障切换；节点离线不会丢失数据库权益。
