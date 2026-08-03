@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\Backup-NivraReleaseKey.ps1"
if not "%ERRORLEVEL%"=="0" pause
