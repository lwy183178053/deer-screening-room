[CmdletBinding(SupportsShouldProcess)]
param(
    [string]$SourceRoot = 'E:\BaiduNetdiskDownload'
)

$ErrorActionPreference = 'Stop'
$MaxFilenameBytes = 240
$LegacyMapFilename = '.deer-media-map.json'
$utf8 = [Text.UTF8Encoding]::new($false)

function Require-Command([string]$Name) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name was not found on PATH."
    }
}

function Get-RelativeMediaPath([string]$Root, [string]$Path) {
    $rootFull = [IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    return $Path.Substring($rootFull.Length).TrimStart('\', '/').Replace('\', '/')
}

function Get-SHA256([string]$Value) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        return $sha.ComputeHash($utf8.GetBytes($Value))
    }
    finally {
        $sha.Dispose()
    }
}

function Get-HexPrefix([byte[]]$Digest, [int]$Length) {
    return (-join ($Digest | ForEach-Object { $_.ToString('x2') })).Substring(0, $Length)
}

function Get-MediaKey([string]$RelativePath) {
    $normalized = $RelativePath.Replace('\', '/').ToLowerInvariant()
    return [Convert]::ToBase64String((Get-SHA256 $normalized)).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function Test-MediaKey([string]$Value) {
    return $Value -match '^[A-Za-z0-9_-]{43}$'
}

function Get-NativePath([string]$Path) {
    $full = if ([IO.Path]::IsPathRooted($Path)) { $Path } else { [IO.Path]::GetFullPath($Path) }
    if (($IsWindows -or $env:OS -eq 'Windows_NT') -and -not $full.StartsWith('\\?\')) {
        return '\\?\' + $full
    }
    return $full
}

function Get-SafeFilename([string]$Filename, [string]$RelativePath) {
    if ($utf8.GetByteCount($Filename) -le $MaxFilenameBytes) { return $Filename }
    $extension = [IO.Path]::GetExtension($Filename).ToLowerInvariant()
    $suffix = '-' + (Get-HexPrefix (Get-SHA256 $RelativePath.Replace('\', '/').ToLowerInvariant()) 8) + $extension
    $budget = $MaxFilenameBytes - $utf8.GetByteCount($suffix)
    if ($budget -lt 1) { throw "Filename suffix exceeds byte budget: $Filename" }
    $builder = [Text.StringBuilder]::new()
    $elements = [Globalization.StringInfo]::GetTextElementEnumerator([IO.Path]::GetFileNameWithoutExtension($Filename))
    while ($elements.MoveNext()) {
        $element = $elements.GetTextElement()
        $candidate = $builder.ToString() + $element
        if ($utf8.GetByteCount($candidate) -gt $budget) { break }
        [void]$builder.Append($element)
    }
    if ($builder.Length -eq 0) { throw "Filename has no safe prefix: $Filename" }
    return $builder.ToString() + $suffix
}

function Read-Probe([string]$Path) {
    $json = & ffprobe -v error -show_entries 'format=duration:format_tags=title,deer_media_key:stream=index,codec_type,codec_name' -of json -- $Path 2>$null
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($json -join ''))) { throw "ffprobe failed: $Path" }
    return ($json -join "`n") | ConvertFrom-Json
}

function Get-LegacyEntries([string]$Root) {
    $result = @{}
    $path = Join-Path $Root $LegacyMapFilename
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { return $result }
    $body = Get-Content -LiteralPath $path -Raw | ConvertFrom-Json
    if ($body.version -and [int]$body.version -ne 1) { throw "Unsupported $LegacyMapFilename version: $($body.version)" }
    if ($body.files) {
        foreach ($property in $body.files.PSObject.Properties) { $result[$property.Name] = $property.Value }
    }
    return $result
}

function Test-ProbeEquivalent($Before, $After, [string]$Title, [string]$MediaKey) {
    if ([string]$After.format.tags.title -ne $Title -or [string]$After.format.tags.deer_media_key -ne $MediaKey) { return $false }
    if (@($Before.streams).Count -ne @($After.streams).Count) { return $false }
    for ($index = 0; $index -lt @($Before.streams).Count; $index++) {
        if ([string]$Before.streams[$index].codec_type -ne [string]$After.streams[$index].codec_type -or
            [string]$Before.streams[$index].codec_name -ne [string]$After.streams[$index].codec_name) { return $false }
    }
    $beforeDuration = [double]$Before.format.duration
    $afterDuration = [double]$After.format.duration
    return [math]::Abs($beforeDuration - $afterDuration) -le 0.05
}

function Set-MediaFile([string]$Root, [IO.FileInfo]$File, [hashtable]$LegacyEntries) {
    $relative = Get-RelativeMediaPath $Root $File.FullName
    $legacy = $LegacyEntries[$File.Name]
    $before = Read-Probe $File.FullName
    $embeddedTitle = [string]$before.format.tags.title
    $embeddedKey = [string]$before.format.tags.deer_media_key
    $title = if (-not [string]::IsNullOrWhiteSpace($embeddedTitle)) { $embeddedTitle.Trim() } elseif ($legacy -and $legacy.title) { [string]$legacy.title } else { $File.BaseName }
    $identityPath = if ($legacy -and $legacy.original_path) { [string]$legacy.original_path } elseif ($legacy -and $legacy.original_name) {
        $studio = if ([string]$legacy.studio -and [string]$legacy.studio -ne '未分类') { [string]$legacy.studio + '/' } else { '' }
        $studio + [string]$legacy.original_name
    } else { $relative }
    $mediaKey = if (Test-MediaKey $embeddedKey) { $embeddedKey } else { Get-MediaKey $identityPath }
    $originalFilename = if ($legacy -and $legacy.original_name) { [IO.Path]::GetFileName([string]$legacy.original_name) } else { $File.Name }
    $targetName = Get-SafeFilename $originalFilename $identityPath
    $target = Join-Path $File.DirectoryName $targetName
    if ($target -ne $File.FullName -and (Test-Path -LiteralPath $target)) { throw "Media target already exists: $target" }

    $metadataMatches = $embeddedTitle -eq $title -and $embeddedKey -eq $mediaKey
    if ($metadataMatches -and $target -eq $File.FullName) { return $false }
    if (-not $PSCmdlet.ShouldProcess($File.FullName, "write MP4 metadata and rename to $targetName")) { return $false }

    if ($metadataMatches) {
        [IO.File]::Move((Get-NativePath $File.FullName), (Get-NativePath $target))
        return $true
    }

    $temporary = Join-Path $File.DirectoryName ('.deer-metadata-' + [guid]::NewGuid().ToString('N') + '.mp4')
    $backup = Join-Path $File.DirectoryName ('.deer-original-' + [guid]::NewGuid().ToString('N') + '.mp4')
    try {
        & ffmpeg -hide_banner -loglevel error -y -i $File.FullName -map 0 -c copy -map_metadata 0 -metadata "title=$title" -metadata "deer_media_key=$mediaKey" -movflags '+faststart+use_metadata_tags' $temporary
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $temporary)) { throw "FFmpeg metadata remux failed: $($File.FullName)" }
        $after = Read-Probe $temporary
        if (-not (Test-ProbeEquivalent $before $after $title $mediaKey)) { throw "Metadata verification failed: $($File.FullName)" }
        $nullDevice = if ($IsWindows -or $env:OS -eq 'Windows_NT') { 'NUL' } else { '/dev/null' }
        & ffmpeg -hide_banner -loglevel error -xerror -i $temporary -map 0 -c copy -f null $nullDevice
        if ($LASTEXITCODE -ne 0) { throw "Packet validation failed: $($File.FullName)" }
        [IO.File]::Move((Get-NativePath $File.FullName), (Get-NativePath $backup))
        try {
            [IO.File]::Move((Get-NativePath $temporary), (Get-NativePath $target))
            [IO.File]::Delete((Get-NativePath $backup))
        }
        catch {
            if ([IO.File]::Exists((Get-NativePath $target))) { [IO.File]::Delete((Get-NativePath $target)) }
            if ([IO.File]::Exists((Get-NativePath $backup))) { [IO.File]::Move((Get-NativePath $backup), (Get-NativePath $File.FullName)) }
            throw
        }
        return $true
    }
    finally {
        if ([IO.File]::Exists((Get-NativePath $temporary))) { [IO.File]::Delete((Get-NativePath $temporary)) }
        if ([IO.File]::Exists((Get-NativePath $backup))) { [IO.File]::Move((Get-NativePath $backup), (Get-NativePath $File.FullName)) }
    }
}

Require-Command 'ffmpeg'
Require-Command 'ffprobe'
$rootFull = [IO.Path]::GetFullPath($SourceRoot).TrimEnd('\', '/')
if (-not (Test-Path -LiteralPath $rootFull -PathType Container)) { throw "Source directory does not exist: $rootFull" }
$legacyEntries = Get-LegacyEntries $rootFull
$files = @(Get-ChildItem -LiteralPath $rootFull -Recurse -File -Filter '*.mp4' -ErrorAction Stop | Sort-Object FullName)
$changed = 0
foreach ($file in $files) { if (Set-MediaFile $rootFull $file $legacyEntries) { $changed++ } }
Write-Host "Media metadata completed: $changed changed, $($files.Count) total." -ForegroundColor Green
