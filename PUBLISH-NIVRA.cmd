@echo off
setlocal
cd /d "%~dp0"
set /p INSTALLER="Dra installasjonsfila hit, eller skriv full sti: "
set INSTALLER=%INSTALLER:"=%
set /p VERSION="Versjon, til dømes 0.1.0-alpha.3: "
set /p CHANNEL="Kanal [alpha/beta/stable] (standard alpha): "
if "%CHANNEL%"=="" set CHANNEL=alpha
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\Publish-NivraRelease.ps1" -Installer "%INSTALLER%" -Version "%VERSION%" -Channel "%CHANNEL%"
set EXITCODE=%ERRORLEVEL%
if not "%EXITCODE%"=="0" (
  echo.
  echo Publiseringa feila. Les feilmeldinga over.
)
pause
exit /b %EXITCODE%
