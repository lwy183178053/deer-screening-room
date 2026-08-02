[CmdletBinding()]
param(
    [ValidateSet('Sample', 'Full', 'New')]
    [string]$Mode = 'Sample',
    [string]$SourceRoot = 'E:\BaiduNetdiskDownload',
    [string]$ViewRoot = 'E:\BaiduNetdiskDownload\.deer-media-view',
    [string]$SampleRoot = 'E:\BaiduNetdiskDownload.av1-samples',
    [string]$MaintenanceLock = ''
)

$ErrorActionPreference = 'Stop'
$TargetLongEdge = 1280
$TargetAudioBitrate = 128000
$VideoExtensions = @('.mp4')
$script:MediaNodeWasRunning = $false
$script:ComposeArgs = @()

function Require-Command([string]$Name) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name was not found on PATH."
    }
}

function Invoke-FFprobe([string]$Path) {
    $json = & ffprobe -v error -show_entries 'format=duration:stream=codec_type,codec_name,width,height,bit_rate,channels,sample_rate' -of json -- $Path 2>$null
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($json -join ''))) {
        throw "ffprobe failed: $Path"
    }
    try {
        return ($json -join "`n") | ConvertFrom-Json
    }
    catch {
        throw "ffprobe returned invalid JSON: $Path"
    }
}

function Get-VideoStream($Probe) {
    $stream = @($Probe.streams | Where-Object { $_.codec_type -eq 'video' } | Select-Object -First 1)
    if ($stream.Count -eq 0 -or [int]$stream[0].width -lt 1 -or [int]$stream[0].height -lt 1) {
        throw 'video stream is missing or has invalid dimensions'
    }
    return $stream[0]
}

function Test-TargetMedia([string]$Path, [object]$Probe) {
    if ([IO.Path]::GetExtension($Path).ToLowerInvariant() -ne '.mp4') { return $false }
    $video = Get-VideoStream $Probe
    if ($video.codec_name -ne 'av1' -or [math]::Max([int]$video.width, [int]$video.height) -gt $TargetLongEdge) { return $false }
    foreach ($audio in @($Probe.streams | Where-Object { $_.codec_type -eq 'audio' })) {
        if ($audio.codec_name -ne 'aac' -or [int]$audio.channels -ne 2 -or [int]$audio.sample_rate -ne 48000) { return $false }
        if ([int]$audio.bit_rate -lt 120000 -or [int]$audio.bit_rate -gt 136000) { return $false }
    }
    return $true
}

function Get-MediaFiles {
    $sourceFull = [IO.Path]::GetFullPath($SourceRoot).TrimEnd('\')
    $viewFull = [IO.Path]::GetFullPath($ViewRoot).TrimEnd('\')
    if (-not (Test-Path -LiteralPath $sourceFull -PathType Container)) {
        throw "Source directory does not exist: $sourceFull"
    }
    return @(Get-ChildItem -LiteralPath $sourceFull -Recurse -File -ErrorAction Stop | Where-Object {
        $relative = $_.FullName.Substring($sourceFull.Length).TrimStart('\')
        $relative -and
        $_.FullName.StartsWith($viewFull + '\', [StringComparison]::OrdinalIgnoreCase) -eq $false -and
        $VideoExtensions -contains $_.Extension.ToLowerInvariant()
    } | Sort-Object FullName)
}

function Get-ScaleFilter($Video) {
    if ([math]::Max([int]$Video.width, [int]$Video.height) -le $TargetLongEdge) {
        return 'scale=trunc(iw/2)*2:trunc(ih/2)*2'
    }
    return "scale=w='if(gt(iw,ih),$TargetLongEdge,-2)':h='if(gt(iw,ih),-2,$TargetLongEdge)'"
}

function Convert-Media([string]$InputPath, [string]$OutputPath) {
    $probe = Invoke-FFprobe $InputPath
    $video = Get-VideoStream $probe
    $filter = Get-ScaleFilter $video
    $outputDirectory = Split-Path -Parent $OutputPath
    New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
    if (Test-Path -LiteralPath $OutputPath) { Remove-Item -LiteralPath $OutputPath -Force }
    $arguments = @(
        '-hide_banner', '-loglevel', 'error', '-y', '-i', $InputPath,
        '-map', '0:v:0', '-map', '0:a?', '-map', '0:s?', '-map_metadata', '0',
        '-vf', $filter, '-c:v', 'av1_nvenc', '-preset', 'p5', '-rc', 'vbr', '-cq', '30', '-b:v', '0',
        '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-ac', '2', '-ar', '48000', '-b:a', '128k',
        '-c:s', 'mov_text', '-movflags', '+faststart', $OutputPath
    )
    & ffmpeg @arguments
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $OutputPath)) {
        throw "FFmpeg AV1 encode failed: $InputPath"
    }
}

function Test-EncodedMedia([string]$Path, [object]$SourceProbe) {
    $probe = Invoke-FFprobe $Path
    if (-not (Test-TargetMedia $Path $probe)) {
        throw "Encoded media does not match AV1 720p AAC target: $Path"
    }
    $sourceDuration = [double]($SourceProbe.format.duration | ForEach-Object { $_ })
    $outputDuration = [double]($probe.format.duration | ForEach-Object { $_ })
    if ($sourceDuration -gt 0 -and ([math]::Abs($sourceDuration - $outputDuration) -gt [math]::Max(3, $sourceDuration * 0.01))) {
        throw "Encoded duration changed unexpectedly: $Path"
    }
    $nullDevice = if ($IsWindows -or $env:OS -eq 'Windows_NT') { 'NUL' } else { '/dev/null' }
    & ffmpeg -hide_banner -loglevel error -xerror -i $Path -map 0:v:0 -map 0:a? -f null $nullDevice
    if ($LASTEXITCODE -ne 0) {
        throw "Encoded media failed full decode: $Path"
    }
    return $probe
}

function Get-ComposeArgs {
    $composeFile = Join-Path $PSScriptRoot '..\deploy\media-node\compose.yaml'
    $envFile = Join-Path $PSScriptRoot '..\deploy\media-node\.env'
    if (-not (Test-Path -LiteralPath $composeFile) -or -not (Test-Path -LiteralPath $envFile)) { return @() }
    $envLines = Get-Content -LiteralPath $envFile | Where-Object { $_ -match '^DEER_COMPOSE_PROJECT_NAME=' }
    $project = if ($envLines) { ($envLines -split '=', 2)[1].Trim() } else { 'deer-screening-room-media' }
    return @('--project-name', $project, '--file', $composeFile, '--env-file', $envFile)
}

function Stop-MediaNode {
    $script:ComposeArgs = Get-ComposeArgs
    if ($script:ComposeArgs.Count -eq 0 -or -not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw 'Docker Compose media-node configuration is required for Full/New mode.'
    }
    $container = (& docker compose @script:ComposeArgs ps -q media-node 2>$null).Trim()
    if (-not $container) { return }
    $running = (& docker inspect -f '{{.State.Running}}' $container 2>$null).Trim()
    if ($running -eq 'true') {
        & docker compose @script:ComposeArgs stop media-node | Out-Host
        if ($LASTEXITCODE -ne 0) { throw 'Could not stop media-node.' }
        $script:MediaNodeWasRunning = $true
    }
}

function Start-MediaNode {
    if (-not $script:MediaNodeWasRunning) { return }
    & docker compose @script:ComposeArgs start media-node | Out-Host
    if ($LASTEXITCODE -ne 0) { throw 'Could not restart media-node.' }
}

function Enter-Maintenance {
    if (-not $MaintenanceLock) { $MaintenanceLock = Join-Path ([IO.Path]::GetFullPath($SourceRoot)) '.deer-media-maintenance.lock' }
    if (Test-Path -LiteralPath $MaintenanceLock) { throw "Maintenance lock already exists: $MaintenanceLock" }
    New-Item -ItemType File -Path $MaintenanceLock -Force | Out-Null
    Stop-MediaNode
    return $MaintenanceLock
}

function Exit-Maintenance([string]$LockPath) {
    try { Start-MediaNode } finally { if ($LockPath -and (Test-Path -LiteralPath $LockPath)) { Remove-Item -LiteralPath $LockPath -Force } }
    & (Join-Path $PSScriptRoot 'prepare-media-view.ps1') -SourceRoot $SourceRoot -ViewRoot $ViewRoot -Quiet
    if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) { throw 'Could not rebuild media compatibility view.' }
}

function Replace-Source([string]$InputPath, [string]$TempPath, [object]$SourceProbe) {
    Test-EncodedMedia $TempPath $SourceProbe | Out-Null
    $backupPath = "$InputPath.av1-original.tmp"
    if (Test-Path -LiteralPath $backupPath) { Remove-Item -LiteralPath $backupPath -Force }
    Move-Item -LiteralPath $InputPath -Destination $backupPath
    try {
        Move-Item -LiteralPath $TempPath -Destination $InputPath
        Remove-Item -LiteralPath $backupPath -Force
    }
    catch {
        if (Test-Path -LiteralPath $InputPath) { Remove-Item -LiteralPath $InputPath -Force }
        if (Test-Path -LiteralPath $backupPath) { Move-Item -LiteralPath $backupPath -Destination $InputPath -Force }
        throw
    }
}

function Invoke-Sample {
    $files = Get-MediaFiles
    if ($files.Count -lt 2) { throw 'At least two MP4 files are required for sample mode.' }
    $probed = @()
    foreach ($file in $files) {
        $probe = Invoke-FFprobe $file.FullName
        $video = Get-VideoStream $probe
        $probed += [pscustomobject]@{ File = $file; Probe = $probe; Video = $video }
    }
    $selected = @($probed | Where-Object { [int]$_.Video.width -ge [int]$_.Video.height } | Select-Object -First 1)
    $second = @($probed | Where-Object { $_.File.FullName -ne $selected[0].File.FullName -and [int]$_.Video.width -lt [int]$_.Video.height } | Select-Object -First 1)
    if ($second.Count -eq 0) { $second = @($probed | Where-Object { $_.File.FullName -ne $selected[0].File.FullName } | Select-Object -First 1) }
    $selected += $second
    New-Item -ItemType Directory -Path $SampleRoot -Force | Out-Null
    $manifest = @()
    for ($index = 0; $index -lt $selected.Count; $index++) {
        $output = Join-Path $SampleRoot ("sample-{0:D2}.mp4" -f ($index + 1))
        Write-Host "Encoding sample $($index + 1)/$($selected.Count): $($selected[$index].File.Name)"
        Convert-Media $selected[$index].File.FullName $output
        $validated = Test-EncodedMedia $output $selected[$index].Probe
        $manifest += [pscustomobject]@{ source = $selected[$index].File.FullName; output = $output; width = $validated.streams[0].width; height = $validated.streams[0].height; duration = $validated.format.duration }
    }
    $manifest | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $SampleRoot 'manifest.json') -Encoding UTF8
    Write-Host "Samples written to $SampleRoot. Original media was not changed."
}

function Invoke-Full([string]$SelectedMode) {
    $lockPath = $null
    try {
        $lockPath = Enter-Maintenance
        $files = Get-MediaFiles
        if ($SelectedMode -eq 'New') { $files = @($files | Where-Object { -not (Test-TargetMedia $_.FullName (Invoke-FFprobe $_.FullName)) }) }
        if ($files.Count -eq 0) { Write-Host 'No media requires AV1 conversion.'; return }
        for ($index = 0; $index -lt $files.Count; $index++) {
            $file = $files[$index]
            $temp = "$($file.FullName).av1.tmp.mp4"
            Write-Host "Encoding $($index + 1)/$($files.Count): $($file.Name)"
            $sourceProbe = Invoke-FFprobe $file.FullName
            Convert-Media $file.FullName $temp
            Replace-Source $file.FullName $temp $sourceProbe
        }
    }
    finally {
        if ($lockPath) { Exit-Maintenance $lockPath }
    }
    Write-Host "AV1 conversion completed: $($files.Count) file(s)."
}

Require-Command 'ffmpeg'
Require-Command 'ffprobe'
if ($Mode -eq 'Sample') { Invoke-Sample } else { Invoke-Full $Mode }
