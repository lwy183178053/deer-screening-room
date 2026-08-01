$ErrorActionPreference = 'Stop'
$base = if ($env:DEER_SMOKE_URL) { $env:DEER_SMOKE_URL.TrimEnd('/') } else { 'http://localhost:8080' }
$health = Invoke-RestMethod "$base/api/v1/health"
if ($health.status -ne 'ok') { throw 'Gateway health check failed' }
$catalog = Invoke-RestMethod "$base/api/v1/videos"
$studios = Invoke-RestMethod "$base/api/v1/studios"
Write-Host "Gateway: $($health.status)"
Write-Host "Videos: $(@($catalog.videos).Count)"
Write-Host "Studios: $(@($studios.studios).Count)"

