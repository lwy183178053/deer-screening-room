# 媒体哈希文件名与标题映射 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task with verification checkpoints.

**Goal:** 在保留工作室分类的前提下，把视频文件标准化为短哈希名，用媒体根目录映射表恢复原始标题，并解决 Windows Docker 超长文件名导致的扫描失败。

**Architecture:** Windows 使用宿主机 PowerShell 处理器，因为 Docker bind mount 在错误发生前无法枚举超长文件名；Linux/NAS 节点扫描器处理可枚举的新文件。两者共享同一 `.deer-media-map.json` 格式，节点只挂载实际媒体目录和海报目录，不再使用旧兼容视图。

**Tech Stack:** Go、PowerShell、Docker Compose、SHA-256、原子文件替换、现有媒体扫描器与目录同步接口。

---

### Task 1: 固化映射格式与哈希命名规则

**Files:**
- Create: `internal/media/name_map.go`
- Modify: `internal/media/scanner.go`
- Test: `internal/media/name_map_test.go`, `internal/media/scanner_test.go`

- [ ] **Step 1: Write the failing tests**

覆盖以下行为：同一个规范化相对路径生成稳定的 `media-<sha256>.<ext>`；映射表可以原子保存和重新加载；扫描器使用映射表中的 `title`，不把哈希文件名显示给用户；目标文件冲突时不覆盖源文件。

- [ ] **Step 2: Run the focused tests and verify they fail**

运行：`go test ./internal/media -run 'Test(MediaName|ScannerMapped)' -count=1`

预期：新测试因映射类型和标准化行为尚不存在而失败。

- [ ] **Step 3: Implement the minimal Go mapping layer**

新增映射结构、加载/保存函数和哈希命名函数。映射文件固定为 `filepath.Join(mediaRoot, ".deer-media-map.json")`，保存使用同目录临时文件和 `os.Rename`。扩展名只取已有视频扩展名并保持小写。

- [ ] **Step 4: Integrate mapped titles into the scanner**

扫描器加载映射；遇到匹配 `media-[0-9a-f]{64}` 的文件时，以映射记录的原始标题和工作室字段构造 `Item`，没有映射时保留文件名作为临时标题并报告可诊断状态。旧 `catalog-cache.json` 只作为媒体探测缓存，不再作为标题来源。

- [ ] **Step 5: Run focused and full Go tests**

运行：`go test ./internal/media -count=1`、`go test ./...`。预期全部通过，且现有 Range、封面和目录版本测试不回退。

- [ ] **Step 6: Commit the mapping layer**

运行：`git add internal/media/name_map.go internal/media/scanner.go internal/media/name_map_test.go internal/media/scanner_test.go && git commit -m "feat: map hashed media names to original titles"`

### Task 2: 增加 Windows 宿主机标准化处理器

**Files:**
- Create: `scripts/normalize-media-names.ps1`
- Modify: `scripts/extract-7z-and-delete.ps1`
- Test: `scripts/normalize-media-names.tests.ps1`

- [ ] **Step 1: Write the failing script checks**

用临时目录创建一个工作室和一个超长视频名，验证处理器会在同一工作室目录生成哈希名、写入 `.deer-media-map.json`，重复运行不会再次改名，非视频文件不受影响。

- [ ] **Step 2: Run the checks and verify they fail**

运行：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/normalize-media-names.tests.ps1`。预期：处理器尚不存在，测试失败。

- [ ] **Step 3: Implement one-shot normalization**

处理器参数固定为 `-SourceRoot`、`-WhatIf` 和 `-IncludeNewOnly`。使用 Windows 原生 `Get-ChildItem -LiteralPath` 枚举视频，排除已符合哈希格式的文件；对每个候选文件计算规范化相对路径哈希，在原目录用 `Move-Item` 完成同目录改名，最后原子更新根目录映射表。

- [ ] **Step 4: Add stable-file protection**

记录文件大小和修改时间，短暂等待后重新读取；两次不一致就跳过该文件并在结果中报告，不能移动仍在写入的视频。

- [ ] **Step 5: Connect the extraction pipeline**

在 `extract-7z-and-delete.ps1` 的转码成功之后调用 `normalize-media-names.ps1 -SourceRoot $SourceRoot`。任何标准化失败都返回非零状态，不删除源文件，并阻止脚本报告“全部完成”。

- [ ] **Step 6: Run script checks**

运行临时目录回归、`extract-7z-and-delete.ps1 -WhatIf` 和 `git diff --check`。确认脚本不会访问或改写仓库外的真实媒体目录。

- [ ] **Step 7: Commit the host processor**

运行：`git add scripts/normalize-media-names.ps1 scripts/normalize-media-names.tests.ps1 scripts/extract-7z-and-delete.ps1 && git commit -m "feat: normalize Windows media filenames"`

### Task 3: 让 Linux/NAS 节点处理可枚举的新文件

**Files:**
- Modify: `internal/media/scanner.go`, `internal/media/node.go`
- Modify: `deploy/media-node/compose.yaml`, `deploy/media-node/compose.ugreen.yaml`, `internal/provisioning/provisioning.go`
- Test: `internal/media/scanner_test.go`, `internal/provisioning/provisioning_test.go`

- [ ] **Step 1: Add a failing scanner test**

在可写临时媒体根目录放入普通文件名视频，扫描后断言源文件变为哈希名、映射表存在、`ResolveMedia` 指向新路径，工作室目录仍保留。

- [ ] **Step 2: Run the test and verify it fails**

运行：`go test ./internal/media -run TestScannerNormalizesNewMedia -count=1`。预期：文件仍是原始名字，测试失败。

- [ ] **Step 3: Implement scan-time normalization for accessible files**

在扫描器探测前处理未标准化的视频；Windows 长文件名在进入容器前由 Task 2 处理，容器内扫描遇到的 `readdirent` 错误仍原样报告，不伪装成成功。标准化后使用新路径继续探测和建立路径索引。

- [ ] **Step 4: Change only the media bind mount to writable**

将节点媒体挂载从 `:ro` 改为 `:rw`，保留容器根文件系统只读、海报目录独立可写和其他安全限制。安装包模板、Windows、本地、绿联 Compose 必须保持一致。

- [ ] **Step 5: Update generated bundle text**

安装包 README 改为说明：工作室目录不变，视频会被标准化为哈希文件名，根目录生成 `.deer-media-map.json`，原始标题仍显示在网站；不再声称“不会重命名视频”。

- [ ] **Step 6: Run Go and Compose checks**

运行：`go test ./internal/media ./internal/provisioning -count=1`、`go test ./...`、`go vet ./...`，以及本地、绿联、registry Compose 的 `config --quiet`。

- [ ] **Step 7: Commit node integration**

运行：`git add internal/media internal/provisioning deploy/media-node && git commit -m "feat: normalize media names during node scans"`

### Task 4: 清理旧兼容视图遗留并验证缓存行为

**Files:**
- Modify: `docs/media-library.md`, `docs/local-archive-extraction.md`, `docs/deployment-fnos.md`, `docs/operations.md`
- Modify: `progress.md`
- Delete only if still tracked: old compatibility-view scripts, view source/tests, stale title-list references

- [ ] **Step 1: Remove obsolete documentation and configuration references**

删除所有“兼容视图”“`/source`”“`catalog-titles.json`”的现行使用说明，只保留历史进度中的事实记录；同步说明映射表和可执行回滚步骤。

- [ ] **Step 2: Add a stale-cache regression check**

使用旧 `catalog-cache.json` 加载后执行一次成功扫描，确认新的映射标题覆盖旧的 `media-...` 标题；失败扫描不得把旧缓存伪装为最新目录。

- [ ] **Step 3: Run repository checks**

运行：`git diff --check`，敏感文件扫描，`go test ./...`、`go test -race ./...`、`go vet ./...` 和全部 Compose 配置检查。

- [ ] **Step 4: Append progress evidence**

在 `progress.md` 末尾记录本轮代码、脚本、测试和回滚点，不记录任何媒体原始标题以外的秘密、Token 或私钥。

- [ ] **Step 5: Commit cleanup and documentation**

运行：`git add docs progress.md && git commit -m "docs: document hashed media filename migration"`

### Task 5: 先做样本迁移，再做全量迁移

**Files outside the repository:**
- Media root: `E:\BaiduNetdiskDownload`
- Runtime cache: local node poster directory and `.deer-media-map.json`

- [ ] **Step 1: Back up runtime metadata and Compose configuration**

停止或暂停当前 `deer-node` 扫描，备份映射表（若存在）、`catalog-cache.json`、海报目录和本地节点 Compose 环境文件；不删除原始视频。

- [ ] **Step 2: Run one horizontal and one vertical sample**

用 `normalize-media-names.ps1 -SourceRoot ... -IncludeNewOnly` 处理两个样本，检查工作室目录、哈希文件、映射标题、封面、播放和 Range `206`。

- [ ] **Step 3: Migrate all 79 files**

维护锁生效后执行全量标准化；每个文件改名成功且映射写入成功后才进入下一个。单文件失败立即停止，保留当前文件和剩余原始文件。

- [ ] **Step 4: Restart only the local media node**

清理旧的海报缓存和过期目录缓存后，启动当前 `deer-node`，等待扫描状态 `ok`，确认 79 个视频和工作室数量恢复。

- [ ] **Step 5: Validate production behavior locally**

验证节点心跳、目录版本、封面、已解锁视频 Range `206`、手机播放和新文件再次加入时的自动标准化。确认 ModelRoute、Redis、MySQL 等其他容器状态不变。

- [ ] **Step 6: Record migration result and rollback point**

在 `progress.md` 追加样本和全量迁移证据。回滚只使用映射表将哈希文件恢复原始文件名、恢复旧 Compose/镜像并重启本地节点，不使用 `docker compose down -v`。
