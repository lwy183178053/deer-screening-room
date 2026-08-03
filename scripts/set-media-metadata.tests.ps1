$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'set-media-metadata.ps1'
$root = Join-Path ([IO.Path]::GetTempPath()) ('deer-media-metadata-test-' + [guid]::NewGuid().ToString('N'))
$studio = Join-Path $root '测试工作室'
New-Item -ItemType Directory -Path $studio -Force | Out-Null

function Read-Tags([string]$Path) {
    $json = & ffprobe -v error -show_entries 'format_tags=title,deer_media_key' -of json -- $Path
    if ($LASTEXITCODE -ne 0) { throw "ffprobe failed: $Path" }
    return ($json -join "`n") | ConvertFrom-Json
}

try {
    $longTitle = ('超长标题' * 55)
    $source = Join-Path $studio ($longTitle + '.mp4')
    & ffmpeg -hide_banner -loglevel error -f lavfi -i 'color=c=black:s=32x32:d=0.2' -c:v libx264 -movflags +faststart -y $source
    if ($LASTEXITCODE -ne 0) { throw 'fixture creation failed' }
    [IO.File]::WriteAllBytes((Join-Path $studio '说明.txt'), [byte[]](1, 2, 3))

    & $scriptPath -SourceRoot $root
    if ($LASTEXITCODE -ne 0) { throw 'metadata script failed' }

    $videos = @(Get-ChildItem -LiteralPath $studio -File -Filter '*.mp4')
    if ($videos.Count -ne 1) { throw "unexpected videos: $($videos.Name -join ', ')" }
    if ([Text.Encoding]::UTF8.GetByteCount($videos[0].Name) -gt 240) { throw 'filename exceeds 240 UTF-8 bytes' }
    if ($videos[0].BaseName -notmatch '-[0-9a-f]{8}$') { throw "missing collision suffix: $($videos[0].Name)" }
    $tags = Read-Tags $videos[0].FullName
    if ($tags.format.tags.title -ne $longTitle) { throw 'title metadata was not preserved' }
    if ([string]$tags.format.tags.deer_media_key -notmatch '^[A-Za-z0-9_-]{43}$') { throw 'stable media key is invalid' }
    $firstName = $videos[0].Name
    $firstKey = [string]$tags.format.tags.deer_media_key

    & $scriptPath -SourceRoot $root
    if ($LASTEXITCODE -ne 0) { throw 'second metadata run failed' }
    $after = @(Get-ChildItem -LiteralPath $studio -File -Filter '*.mp4')
    $afterTags = Read-Tags $after[0].FullName
    if ($after.Count -ne 1 -or $after[0].Name -ne $firstName -or [string]$afterTags.format.tags.deer_media_key -ne $firstKey) {
        throw 'metadata processing is not idempotent'
    }
    if (-not (Test-Path -LiteralPath (Join-Path $studio '说明.txt'))) { throw 'non-video file was changed' }

    $boundaryRoot = Join-Path $root 'boundary'
    $boundaryStudio = Join-Path $boundaryRoot 'studio'
    New-Item -ItemType Directory -Path $boundaryStudio -Force | Out-Null
    $boundaryTitle = 'a' * 236
    $boundaryName = $boundaryTitle + '.mp4'
    $boundarySource = Join-Path $boundaryStudio $boundaryName
    & ffmpeg -hide_banner -loglevel error -f lavfi -i 'color=c=black:s=32x32:d=0.2' -c:v libx264 -movflags +faststart -y $boundarySource
    if ($LASTEXITCODE -ne 0) { throw 'boundary fixture creation failed' }
    if ([Text.Encoding]::UTF8.GetByteCount($boundaryName) -ne 240) { throw 'boundary fixture is not exactly 240 bytes' }
    & $scriptPath -SourceRoot $boundaryRoot
    if ($LASTEXITCODE -ne 0) { throw 'boundary metadata run failed' }
    $boundaryVideo = @(Get-ChildItem -LiteralPath $boundaryStudio -File -Filter '*.mp4')
    if ($boundaryVideo.Count -ne 1 -or $boundaryVideo[0].Name -ne $boundaryName) { throw '240-byte filename was shortened' }
    $boundaryTags = Read-Tags $boundaryVideo[0].FullName
    if ([string]$boundaryTags.format.tags.title -ne $boundaryTitle) { throw 'boundary title metadata mismatch' }

    $conflictRoot = Join-Path $root 'conflict'
    $conflictStudio = Join-Path $conflictRoot '工作室'
    New-Item -ItemType Directory -Path $conflictStudio -Force | Out-Null
    $sourceName = 'source.mp4'
    $targetName = 'target.mp4'
    $conflictSource = Join-Path $conflictStudio $sourceName
    $conflictTarget = Join-Path $conflictStudio $targetName
    & ffmpeg -hide_banner -loglevel error -f lavfi -i 'color=c=black:s=32x32:d=0.2' -c:v libx264 -movflags +faststart -y $conflictSource
    if ($LASTEXITCODE -ne 0) { throw 'conflict fixture creation failed' }
    [IO.File]::WriteAllBytes($conflictTarget, [byte[]](9, 8, 7))
    $legacyMap = [ordered]@{
        version = 1
        files = [ordered]@{
            $sourceName = [ordered]@{ studio = '工作室'; original_name = $targetName; original_path = '工作室/target.mp4'; title = '目标标题' }
        }
    }
    [IO.File]::WriteAllText((Join-Path $conflictRoot '.deer-media-map.json'), ($legacyMap | ConvertTo-Json -Depth 5), [Text.UTF8Encoding]::new($false))
    $conflictFailed = $false
    try { & $scriptPath -SourceRoot $conflictRoot } catch { $conflictFailed = $true }
    if (-not $conflictFailed) { throw 'destination conflict was accepted' }
    if (-not (Test-Path -LiteralPath $conflictSource)) { throw 'conflict source was removed' }
    if ([Convert]::ToBase64String([IO.File]::ReadAllBytes($conflictTarget)) -ne 'CQgH') { throw 'conflict target was overwritten' }

    Write-Host 'set-media-metadata tests passed.' -ForegroundColor Green
}
finally {
    if (Test-Path -LiteralPath $root) { Remove-Item -LiteralPath $root -Recurse -Force }
}
