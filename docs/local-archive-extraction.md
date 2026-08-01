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

After adding videos on Windows, keep `scripts\watch-media-view.cmd` running when the Docker media node uses the compatibility view. It updates short hard links automatically while preserving the original titles in the node catalog.
