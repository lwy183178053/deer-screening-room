$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'normalize-media-names.ps1'
$root = Join-Path ([IO.Path]::GetTempPath()) ('deer-media-name-test-' + [guid]::NewGuid().ToString('N'))
$studio = Join-Path $root '悠米'
New-Item -ItemType Directory -Path $studio -Force | Out-Null

try {
    $longName = (('长名' * 90) + '.mp4')
    $source = Join-Path $studio $longName
    [IO.File]::WriteAllBytes($source, [byte[]](1, 2, 3))
    [IO.File]::WriteAllBytes((Join-Path $studio '说明.txt'), [byte[]](4, 5, 6))

    & $scriptPath -SourceRoot $root

    $videos = @(Get-ChildItem -LiteralPath $studio -File | Where-Object Extension -eq '.mp4')
    if ($videos.Count -ne 1 -or $videos[0].Name -notmatch '^media-[0-9a-f]{64}\.mp4$') {
        throw "unexpected normalized files: $($videos.Name -join ', ')"
    }
    $mapPath = Join-Path $root '.deer-media-map.json'
    $map = Get-Content -LiteralPath $mapPath -Raw | ConvertFrom-Json
    $entry = $map.files.($videos[0].Name)
    if (-not $entry -or $entry.title -ne [IO.Path]::GetFileNameWithoutExtension($longName) -or $entry.studio -ne '悠米') {
        throw 'mapping entry did not preserve the original title'
    }
    if (-not (Test-Path -LiteralPath (Join-Path $studio '说明.txt'))) {
        throw 'non-video file was changed'
    }

    & $scriptPath -SourceRoot $root
    $videosAgain = @(Get-ChildItem -LiteralPath $studio -File | Where-Object Extension -eq '.mp4')
    if ($videosAgain.Count -ne 1 -or $videosAgain[0].Name -ne $videos[0].Name) {
        throw 'normalization was not idempotent'
    }

    Write-Host 'normalize-media-names tests passed.' -ForegroundColor Green
}
finally {
    if (Test-Path -LiteralPath $root) { Remove-Item -LiteralPath $root -Recurse -Force }
}
