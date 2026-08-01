@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0extract-7z-and-delete.ps1" %*
set "exitCode=%ERRORLEVEL%"
echo.
if not "%exitCode%"=="0" echo Some archives were kept because extraction failed.
if "%exitCode%"=="0" echo Finished.
pause
exit /b %exitCode%
