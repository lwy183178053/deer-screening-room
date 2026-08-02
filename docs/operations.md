# Operations and backup

每天备份 PostgreSQL：

```bash
docker compose exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > deerroom-$(date +%F).dump
```

每月至少做一次空库恢复演练。需要备份的运行数据包括 PostgreSQL、云端/节点 WireGuard 配置和节点 `posters/`；视频源由宿主机自身策略负责。封面缓存丢失可重新扫描生成，数据库和兑换码已导出的原始 TXT 不可由哈希恢复。

监控重点：Gateway `/api/v1/health`、Caddy HTTPS、coturn 进程与 `3478` 监听、TURN relay 端口流量、节点最后心跳与扫描状态、节点 FFmpeg 进程、磁盘余量、兑换码异常和管理员调账审计。节点超过 90 秒没有心跳即视为离线并从公开目录隐藏。Gateway 每小时清理过期登录会话和播放会话；节点在 PeerConnection 关闭、FFmpeg 结束或六小时 TTL 到期时清理播放资源。

升级前先备份数据库与 `.env`，记录当前 Compose 容器 ID、启动时间和 Caddy 配置备份，再执行定向 `docker compose build` 与 `up -d`。数据库迁移在 Gateway 启动时按顺序执行；失败时 Gateway 不启动。升级后同时验证 TURN-only 与允许直连两种播放模式，并确认服务器既有容器没有重启。首帧延迟排查重点是浏览器 ICE gathering 是否在 1.5 秒上限内完成、节点是否在 PeerConnection connected 后才启动 FFmpeg，以及兼容视频是否触发实时 H.264 转码；直连诊断使用浏览器 `getStats()` 检查 candidate pair，`host`/`srflx` 为直连候选，`relay` 为 TURN，回落 TURN 属于正常结果。
