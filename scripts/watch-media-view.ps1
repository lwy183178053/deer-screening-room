[CmdletBinding()]
param(
    [string]$SourceRoot = 'E:\BaiduNetdiskDownload',
    [string]$ViewRoot = 'E:\BaiduNetdiskDownload\.deer-media-view',
    [int]$IntervalSeconds = 20
)

$ErrorActionPreference = 'Continue'
$scriptPath = Join-Path $PSScriptRoot 'prepare-media-view.ps1'
while ($true) {
    & $scriptPath -SourceRoot $SourceRoot -ViewRoot $ViewRoot -Quiet
    Start-Sleep -Seconds ([Math]::Max(5, $IntervalSeconds))
}
