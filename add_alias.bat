@echo off
setlocal

where powershell >nul 2>nul
if errorlevel 1 (
    echo redfetch -^> powershell not found
    exit /b 1
)

set "SCRIPT=%~dp0add_alias.ps1"
if not exist "%SCRIPT%" (
    set "SCRIPT=%TEMP%\redfetch-add_alias.ps1"
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -UseBasicParsing -Uri 'https://raw.githubusercontent.com/TechnoRed2026/redfetch/main/add_alias.ps1' -OutFile '%SCRIPT%'"
    if errorlevel 1 exit /b %errorlevel%
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" %*
exit /b %errorlevel%
