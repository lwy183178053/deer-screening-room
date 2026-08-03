# Local archive extraction

`scripts/extract-7z-and-delete.cmd` is a Windows one-click helper for the local media folder.

By default it processes all `.7z` files below `E:\BaiduNetdiskDownload\悠米`. The password is read from `说明.txt`, so the password is not stored in the script. Each archive is tested and extracted directly into its containing directory; existing same-name files are overwritten by 7-Zip. The source archive is deleted only after extraction succeeds.

Requirements:

- 7-Zip must be installed and `7z.exe` must be on `PATH`, or installed in the standard `C:\Program Files\7-Zip` location.
- The account running the script must have write access to the archive directory.

Run `extract-7z-and-delete.cmd` by double-clicking it. To preview the files without extracting or deleting anything, run:

```powershell
.\scripts\extract-7z-and-delete.ps1 -WhatIf
```

To process another directory:

```powershell
.\scripts\extract-7z-and-delete.ps1 -ArchiveRoot 'D:\media\悠米'
```

If an archive fails its password/integrity test or extraction, it is kept and the script continues with the remaining archives. A failed direct extraction may leave partial files beside the retained archive; rerun it after fixing the problem.

After adding videos on Windows, the extraction script runs `scripts\transcode-av1-720p.ps1 -Mode New` after the complete archive batch succeeds. It pauses only the media-node service, validates each AV1 output, writes the MP4 `title` and `deer_media_key`, and shortens filenames above 240 UTF-8 bytes. A failed conversion or metadata update keeps the source file and stops that batch.

The conversion script supports a non-destructive sample run and the full migration:

```powershell
.\scripts\transcode-av1-720p.ps1 -Mode Sample
.\scripts\transcode-av1-720p.ps1 -Mode Full
```

Sample output is written to `E:\BaiduNetdiskDownload.av1-samples`; it never changes source media. Full/New mode requires FFmpeg with `av1_nvenc` and `ffprobe` on `PATH`, and uses AAC-LC 48 kHz stereo at 128 kbps.
