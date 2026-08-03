$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
docker build --tag deerroom-app:dev .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
docker build --tag deerroom-web:dev .\web
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
docker compose up -d --no-build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
docker compose ps
