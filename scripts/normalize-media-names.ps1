[CmdletBinding(SupportsShouldProcess)]
param(
    [string]$SourceRoot = 'E:\BaiduNetdiskDownload',
    [switch]$IncludeNewOnly
)

$ErrorActionPreference = 'Stop'
$VideoExtensions = @('.mp4', '.m4v', '.mov', '.mkv', '.webm', '.avi')
$HashedNamePattern = '^media-[0-9a-f]{64}\.[a-z0-9]+$'
$MapFilename = '.deer-media-map.json'

function Get-RelativeMediaPath([string]$Root, [string]$Path) {
    $rootFull = [IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    return $Path.Substring($rootFull.Length).TrimStart('\', '/').Replace('\', '/')
}

function Get-HashedMediaFilename([string]$RelativePath) {
    $normalized = $RelativePath.Replace('\', '/').ToLowerInvariant()
    $bytes = [Text.Encoding]::UTF8.GetBytes($normalized)
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $digest = $sha.ComputeHash($bytes)
    }
    finally {
        $sha.Dispose()
    }
    $hex = -join ($digest | ForEach-Object { $_.ToString('x2') })
    return 'media-' + $hex + ([IO.Path]::GetExtension($RelativePath).ToLowerInvariant())
}

function Read-MediaMap([string]$Root) {
    $map = [ordered]@{ version = 1; files = [ordered]@{} }
    $path = Join-Path $Root $MapFilename
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        return $map
    }
    $body = Get-Content -LiteralPath $path -Raw | ConvertFrom-Json
    if ($body.version -and [int]$body.version -ne 1) {
        throw "Unsupported $MapFilename version: $($body.version)"
    }
    if ($body.files) {
        foreach ($property in $body.files.PSObject.Properties) {
            $map.files[$property.Name] = [ordered]@{
                studio = [string]$property.Value.studio
                original_name = [string]$property.Value.original_name
                title = [string]$property.Value.title
            }
        }
    }
    return $map
}

function Write-MediaMap([string]$Root, [hashtable]$Map) {
    $path = Join-Path $Root $MapFilename
    $temp = Join-Path $Root ($MapFilename + '.tmp-' + [guid]::NewGuid().ToString('N'))
    $json = $Map | ConvertTo-Json -Depth 6
    try {
        [IO.File]::WriteAllText($temp, $json, [Text.UTF8Encoding]::new($false))
        Move-Item -LiteralPath $temp -Destination $path -Force
    }
    finally {
        if (Test-Path -LiteralPath $temp) { Remove-Item -LiteralPath $temp -Force }
    }
}

function Test-MediaStable([IO.FileInfo]$File) {
    $before = Get-Item -LiteralPath $File.FullName -Force
    Start-Sleep -Milliseconds 250
    $after = Get-Item -LiteralPath $File.FullName -Force
    return $before.Length -eq $after.Length -and $before.LastWriteTimeUtc -eq $after.LastWriteTimeUtc
}

function Normalize-MediaFile([hashtable]$Map, [string]$Root, [IO.FileInfo]$File) {
    if ($File.Name -match $HashedNamePattern) {
        return $false
    }
    if ($VideoExtensions -notcontains $File.Extension.ToLowerInvariant()) {
        return $false
    }
    if (-not (Test-MediaStable $File)) {
        Write-Warning "Skipped changing file: it is still being written: $($File.FullName)"
        return $false
    }

    $relative = Get-RelativeMediaPath $Root $File.FullName
    $targetName = Get-HashedMediaFilename $relative
    $target = Join-Path $File.DirectoryName $targetName
    if (Test-Path -LiteralPath $target) {
        throw "Media target already exists: $target"
    }
    if ($WhatIfPreference) {
        Write-Host "Would rename: $($File.FullName) -> $target"
        return $false
    }

    $key = [IO.Path]::GetFileName($target)
    $hadPrevious = $Map.files.Contains($key)
    $previous = if ($hadPrevious) { $Map.files[$key] } else { $null }
    $parts = $relative -split '/'
    $studio = if ($parts.Count -gt 1) { $parts[0] } else { '未分类' }
    $entry = [ordered]@{
        studio = $studio
        original_name = $File.Name
        title = [IO.Path]::GetFileNameWithoutExtension($File.Name)
    }

    Move-Item -LiteralPath $File.FullName -Destination $target
    try {
        $Map.files[$key] = $entry
        Write-MediaMap $Root $Map
    }
    catch {
        if ($hadPrevious) { $Map.files[$key] = $previous } else { $Map.files.Remove($key) }
        if (Test-Path -LiteralPath $target) { Move-Item -LiteralPath $target -Destination $File.FullName -Force }
        throw
    }
    Write-Host "Renamed: $relative -> $($parts[0..($parts.Count - 2)] -join '/')/$targetName"
    return $true
}

$rootFull = [IO.Path]::GetFullPath($SourceRoot).TrimEnd('\', '/')
if (-not (Test-Path -LiteralPath $rootFull -PathType Container)) {
    throw "Source directory does not exist: $rootFull"
}

$map = Read-MediaMap $rootFull
$files = @(Get-ChildItem -LiteralPath $rootFull -Recurse -File -ErrorAction Stop | Sort-Object FullName)
$changed = 0
foreach ($file in $files) {
    if (Normalize-MediaFile $map $rootFull $file) { $changed++ }
}
if ($WhatIfPreference) {
    Write-Host 'Dry run completed.'
} else {
    Write-Host "Media filename normalization completed: $changed file(s)." -ForegroundColor Green
}
