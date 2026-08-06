$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'transcode-av1-720p.ps1'
$root = Join-Path ([IO.Path]::GetTempPath()) ('deer-transcode-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $root -Force | Out-Null

try {
    $source = Join-Path $root 'silent-audio.mp4'
    & ffmpeg -hide_banner -loglevel error -f lavfi -i 'color=c=black:s=320x180:d=1' -f lavfi -i 'anullsrc=r=48000:cl=stereo' -shortest -c:v libx264 -c:a aac -b:a 16k -movflags +faststart -y $source
    if ($LASTEXITCODE -ne 0) { throw 'fixture creation failed' }

    & $scriptPath -Mode New -SourceRoot $root
    if ($LASTEXITCODE -ne 0) { throw 'transcode script failed' }

    $json = & ffprobe -v error -show_entries 'stream=codec_type,codec_name,bit_rate,channels,sample_rate' -of json -- $source
    if ($LASTEXITCODE -ne 0) { throw 'ffprobe failed' }
    $probe = ($json -join "`n") | ConvertFrom-Json
    $video = @($probe.streams | Where-Object { $_.codec_type -eq 'video' })[0]
    $audio = @($probe.streams | Where-Object { $_.codec_type -eq 'audio' })[0]
    if ($video.codec_name -ne 'av1') { throw 'video was not converted to AV1' }
    if ($audio.codec_name -ne 'aac' -or [int]$audio.channels -ne 2 -or [int]$audio.sample_rate -ne 48000) {
        throw 'audio target does not match AAC stereo 48 kHz'
    }
    if ([int]$audio.bit_rate -ge 80000) { throw 'fixture did not produce low measured audio bitrate' }

    Write-Host 'transcode AV1 720p tests passed.' -ForegroundColor Green
}
finally {
    if (Test-Path -LiteralPath $root) { Remove-Item -LiteralPath $root -Recurse -Force }
}
