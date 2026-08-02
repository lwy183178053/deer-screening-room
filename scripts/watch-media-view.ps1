[CmdletBinding()]
param(
    [string]$SourceRoot = 'E:\BaiduNetdiskDownload',
    [string]$ViewRoot = 'E:\BaiduNetdiskDownload\.deer-media-view',
    [int]$IntervalSeconds = 20,
    [string]$MaintenanceLock = ''
)

$ErrorActionPreference = 'Continue'
$scriptPath = Join-Path $PSScriptRoot 'prepare-media-view.ps1'
if (-not $MaintenanceLock) { $MaintenanceLock = Join-Path ([IO.Path]::GetFullPath($SourceRoot)) '.deer-media-maintenance.lock' }
while ($true) {
    if (Test-Path -LiteralPath $MaintenanceLock) {
        Start-Sleep -Seconds ([Math]::Max(5, $IntervalSeconds))
        continue
    }
    & $scriptPath -SourceRoot $SourceRoot -ViewRoot $ViewRoot -Quiet
    Start-Sleep -Seconds ([Math]::Max(5, $IntervalSeconds))
}
