# Operations and backup

每天备份 PostgreSQL：

```bash
docker compose exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > deerroom-$(date +%F).dump
```

每月至少做一次空库恢复演练。需要备份的运行数据包括 PostgreSQL、云端/节点 WireGuard 配置和节点 `posters/`；视频源由宿主机自身策略负责。封面缓存丢失可重新扫描生成，数据库和兑换码已导出的原始 TXT 不可由哈希恢复。

监控重点：Gateway `/api/v1/health`、Caddy HTTPS、节点最后心跳与扫描状态、磁盘余量、兑换码异常、管理员调账审计和当前播放数。节点超过 90 秒没有心跳即视为离线并从公开目录隐藏。删除节点时确认 WireGuard Peer、目录索引、视频权益和播放会话均已清理；源视频文件不由云端删除。Gateway 每小时清理过期登录会话和播放会话。

升级前先备份数据库，再执行 `docker compose build` 和 `docker compose up -d`。数据库迁移在 Gateway 启动时按顺序执行；失败时 Gateway 不启动。

Windows 媒体迁移使用 `scripts\transcode-av1-720p.ps1` 和 `scripts\normalize-media-names.ps1`。Sample 模式只生成样本；Full/New 模式会创建 `.deer-media-maintenance.lock`，暂停当前小鹿 media-node，逐个用 `av1_nvenc` 编码并通过 `ffprobe` 与完整解码校验，成功后才原子替换和删除原片，再在原工作室目录内生成哈希文件名和 `.deer-media-map.json`。脚本失败时保留失败文件的原片；不要在 Full 模式运行期间手动删除维护锁或媒体文件。节点扫描器在命中旧缓存时也会重新计算 AV1/AAC 的播放兼容性，避免编码迁移后沿用旧的 `unsupported` 标记。
