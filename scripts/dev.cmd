@echo off
REM Windows launcher for scripts\dev.ps1 (CGO + MinGW + wails dev)
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0dev.ps1" %*
exit /b %ERRORLEVEL%
