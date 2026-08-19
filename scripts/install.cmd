@echo off
setlocal
rem Install wheretoken from cmd.exe. irm/iex are PowerShell-only.
rem   powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex"
set "SCRIPT=%~dp0install.ps1"
where powershell >nul 2>nul
if errorlevel 1 (
  echo wheretoken: need powershell.exe on PATH
  exit /b 1
)
if exist "%SCRIPT%" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" %*
) else (
  powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex"
)
exit /b %ERRORLEVEL%
