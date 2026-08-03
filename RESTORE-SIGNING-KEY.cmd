@echo off
setlocal
cd /d "%~dp0"
set /p BACKUP="Dra .nivrakey-fila hit, eller skriv full sti: "
set BACKUP=%BACKUP:"=%
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\Restore-NivraReleaseKey.ps1" -Backup "%BACKUP%"
pause
