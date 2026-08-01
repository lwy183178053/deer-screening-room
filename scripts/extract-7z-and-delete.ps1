[CmdletBinding(SupportsShouldProcess)]
param(
    [string]$ArchiveRoot = 'E:\BaiduNetdiskDownload\悠米'
)

$ErrorActionPreference = 'Stop'

function Find-SevenZip {
    $command = Get-Command 7z.exe -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    $candidates = @(
        (Join-Path $env:ProgramFiles '7-Zip\7z.exe'),
        (Join-Path ${env:ProgramFiles(x86)} '7-Zip\7z.exe')
    )

    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) {
            return $candidate
        }
    }

    throw '7z.exe was not found. Install 7-Zip and run this script again.'
}

function Read-ArchivePassword([string]$Root) {
    $instructionFiles = @(
        (Join-Path $Root '说明.txt'),
        (Join-Path (Split-Path -Parent $Root) '说明.txt')
    ) | Select-Object -Unique

    foreach ($instructionFile in $instructionFiles) {
        if (-not (Test-Path -LiteralPath $instructionFile)) {
            continue
        }

        $match = Select-String -LiteralPath $instructionFile -Pattern '解压密码\s*[:：]\s*(\S+)' | Select-Object -First 1
        if ($match) {
            return $match.Matches[0].Groups[1].Value
        }
    }

    throw 'Could not find an extraction password in 说明.txt.'
}

if (-not (Test-Path -LiteralPath $ArchiveRoot -PathType Container)) {
    throw "Archive directory does not exist: $ArchiveRoot"
}

$archives = @(Get-ChildItem -LiteralPath $ArchiveRoot -Recurse -File -Filter '*.7z' | Sort-Object FullName)

if ($archives.Count -eq 0) {
    Write-Host "No .7z files found under $ArchiveRoot"
    exit 0
}

Write-Host "Found $($archives.Count) archive(s) under $ArchiveRoot"
if ($WhatIfPreference) {
    $archives | ForEach-Object { Write-Host "Would extract and delete: $($_.FullName)" }
    exit 0
}

$sevenZip = Find-SevenZip
$password = Read-ArchivePassword $ArchiveRoot
$failed = 0
foreach ($archive in $archives) {
    try {
        Write-Host "Testing: $($archive.Name)"
        & $sevenZip 't' $archive.FullName "-p$password" '-y' | Out-Host
        if ($LASTEXITCODE -ne 0) {
            throw "Archive test failed (exit code $LASTEXITCODE)."
        }

        $archiveDirectory = Split-Path -Parent $archive.FullName
        Write-Host "Extracting: $($archive.Name)"
        & $sevenZip 'x' $archive.FullName "-o$archiveDirectory" "-p$password" '-y' '-aoa' | Out-Host
        if ($LASTEXITCODE -ne 0) {
            throw "Extraction failed (exit code $LASTEXITCODE)."
        }

        Remove-Item -LiteralPath $archive.FullName -Force
        Write-Host "Completed and deleted: $($archive.Name)" -ForegroundColor Green
    }
    catch {
        $failed++
        Write-Warning "Kept archive after failure: $($archive.FullName). $($_.Exception.Message)"
    }
}

if ($failed -gt 0) {
    throw "$failed archive(s) failed. Failed archives were not deleted."
}

Write-Host 'All archives were extracted successfully and deleted.' -ForegroundColor Green
