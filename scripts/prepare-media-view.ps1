[CmdletBinding()]
param(
    [string]$SourceRoot = 'E:\BaiduNetdiskDownload',
    [string]$ViewRoot = 'E:\BaiduNetdiskDownload\.deer-media-view',
    [switch]$Quiet
)

$ErrorActionPreference = 'Stop'

if (-not ('DeerMedia.NativeHardLink' -as [type])) {
    Add-Type @'
using System;
using System.Runtime.InteropServices;
namespace DeerMedia {
    public static class NativeHardLink {
        [DllImport("Kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern bool CreateHardLink(string fileName, string existingFileName, IntPtr securityAttributes);
    }
}
'@
}

function Write-Log([string]$Message) {
    if (-not $Quiet) { Write-Host $Message }
}

function Get-RelativePath([string]$Root, [string]$Path) {
    $prefix = $Root.TrimEnd('\') + '\'
    return $Path.Substring($prefix.Length).Replace('\', '/')
}

function Get-TextHash([string]$Value) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [Text.Encoding]::UTF8.GetBytes($Value)
        return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $sha.Dispose()
    }
}

function Get-WindowsPath([string]$Path) {
    $full = [IO.Path]::GetFullPath($Path)
    if ($full.StartsWith('\\?\')) { return $full }
    return '\\?\' + $full
}

function Get-FileSignature($Files, [string]$Root) {
    $parts = foreach ($file in $Files) {
        $relative = Get-RelativePath $Root $file.FullName
        "$relative|$($file.Length)|$($file.LastWriteTimeUtc.Ticks)"
    }
    return Get-TextHash (($parts | Sort-Object) -join "`n")
}

$sourceFull = [IO.Path]::GetFullPath($SourceRoot).TrimEnd('\')
$viewFull = [IO.Path]::GetFullPath($ViewRoot).TrimEnd('\')
if (-not (Test-Path -LiteralPath $sourceFull -PathType Container)) {
    throw "Source directory does not exist: $sourceFull"
}
if ($viewFull.StartsWith($sourceFull + '\', [StringComparison]::OrdinalIgnoreCase) -eq $false) {
    throw 'ViewRoot must be inside SourceRoot so Docker can use one host media tree.'
}

New-Item -ItemType Directory -Path $viewFull -Force | Out-Null
$viewPrefix = $viewFull + '\'
$files = @(Get-ChildItem -LiteralPath $sourceFull -Recurse -File -ErrorAction Stop | Where-Object {
    $_.FullName.StartsWith($viewPrefix, [StringComparison]::OrdinalIgnoreCase) -eq $false -and
    @('.mp4', '.m4v', '.mov', '.mkv', '.webm', '.avi') -contains $_.Extension.ToLowerInvariant()
} | Sort-Object FullName)
$signature = Get-FileSignature $files $sourceFull
$statePath = Join-Path $viewFull '.deer-media-state'
if ((Test-Path -LiteralPath $statePath) -and ((Get-Content -LiteralPath $statePath -Raw).Trim() -eq $signature)) {
    exit 0
}

$manifestPath = Join-Path $viewFull 'catalog-titles.json'
$oldManifestPath = $manifestPath
if (Test-Path -LiteralPath $oldManifestPath) {
    try {
        $oldManifest = Get-Content -LiteralPath $oldManifestPath -Raw | ConvertFrom-Json
        foreach ($entry in @($oldManifest.entries)) {
            if ($entry.path) {
                $oldPath = Join-Path $viewFull ($entry.path.Replace('/', '\'))
                if (Test-Path -LiteralPath $oldPath) { Remove-Item -LiteralPath $oldPath -Force }
            }
        }
    }
    catch {
        Write-Log 'Existing media manifest could not be read; rebuilding the view.'
    }
}

# Remove files left by an interrupted first build or a previous manifest.
Get-ChildItem -LiteralPath $viewFull -Recurse -File -ErrorAction SilentlyContinue | Where-Object {
    @('.mp4', '.m4v', '.mov', '.mkv', '.webm', '.avi') -contains $_.Extension.ToLowerInvariant()
} | Remove-Item -Force

$entries = @()
foreach ($file in $files) {
    $relative = Get-RelativePath $sourceFull $file.FullName
    $relativeDirectory = Split-Path -Parent $relative
    $fileName = Split-Path -Leaf $relative
    $utf8Length = [Text.Encoding]::UTF8.GetByteCount($fileName)
    if ($utf8Length -gt 240) {
        $alias = 'media-' + (Get-TextHash $relative) + $file.Extension.ToLowerInvariant()
    }
    else {
        $alias = $fileName
    }
    $viewRelative = if ($relativeDirectory) { (Join-Path $relativeDirectory $alias).Replace('\', '/') } else { $alias }
    $destination = Join-Path $viewFull ($viewRelative.Replace('/', '\'))
    New-Item -ItemType Directory -Path (Split-Path -Parent $destination) -Force | Out-Null
    if (-not [DeerMedia.NativeHardLink]::CreateHardLink((Get-WindowsPath $destination), (Get-WindowsPath $file.FullName), [IntPtr]::Zero)) {
        $errorCode = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
        throw "Could not create hard link for $relative (Win32 error $errorCode)."
    }
    $entries += [pscustomobject]@{ path = $viewRelative; title = [IO.Path]::GetFileNameWithoutExtension($fileName) }
}

$manifest = [pscustomobject]@{ version = 1; entries = $entries }
$manifestTemp = "$manifestPath.tmp"
$manifest | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $manifestTemp -Encoding UTF8
Move-Item -LiteralPath $manifestTemp -Destination $manifestPath -Force
Set-Content -LiteralPath $statePath -Value $signature -Encoding ASCII
Write-Log "Media view updated: $($files.Count) file(s); $(@($entries | Where-Object { $_.path -match '/media-[0-9a-f]+\.' }).Count) long name(s) aliased."
