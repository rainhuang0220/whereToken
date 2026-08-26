@echo off
setlocal EnableExtensions EnableDelayedExpansion
rem cmd.exe installer. No PowerShell required.
rem   curl.exe -fsSL -o %TEMP%\wt-install.cmd https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.cmd && %TEMP%\wt-install.cmd

if not defined WHERETOKEN_REPO set "WHERETOKEN_REPO=rainhuang0220/whereToken"
if not defined PREFIX set "PREFIX=%LOCALAPPDATA%\whereToken"
if not defined BIN_DIR set "BIN_DIR=%PREFIX%\bin"

set "ARCH=%PROCESSOR_ARCHITECTURE%"
if defined PROCESSOR_ARCHITEW6432 set "ARCH=%PROCESSOR_ARCHITEW6432%"
set "GOARCH="
if /i "%ARCH%"=="AMD64" set "GOARCH=amd64"
if /i "%ARCH%"=="ARM64" set "GOARCH=arm64"
if not defined GOARCH (
  echo wheretoken: unsupported arch %ARCH%
  exit /b 1
)

where curl.exe >nul 2>nul
if errorlevel 1 (
  echo wheretoken: need curl.exe on PATH
  exit /b 1
)
where tar.exe >nul 2>nul
if errorlevel 1 (
  echo wheretoken: need tar.exe on PATH
  exit /b 1
)
where certutil.exe >nul 2>nul
if errorlevel 1 (
  echo wheretoken: need certutil.exe on PATH
  exit /b 1
)

set "BASE=https://github.com/%WHERETOKEN_REPO%/releases/latest/download"
if defined WHERETOKEN_RELEASE_URL (
  set "BASE=%WHERETOKEN_RELEASE_URL%"
  if "!BASE:~-1!"=="/" set "BASE=!BASE:~0,-1!"
) else if defined WHERETOKEN_VERSION (
  set "VER=%WHERETOKEN_VERSION%"
  if /i "!VER:~0,1!"=="v" set "VER=!VER:~1!"
  set "BASE=https://github.com/%WHERETOKEN_REPO%/releases/download/v!VER!"
)

set "ASSET=wheretoken_windows_%GOARCH%.zip"
set "WORKDIR=%TEMP%\wheretoken-install-%RANDOM%%RANDOM%"
mkdir "%WORKDIR%" || exit /b 1

curl.exe -fsSL -o "%WORKDIR%\%ASSET%" "%BASE%/%ASSET%"
if errorlevel 1 (
  echo wheretoken: download failed
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
curl.exe -fsSL -o "%WORKDIR%\checksums.txt" "%BASE%/checksums.txt"
if errorlevel 1 (
  echo wheretoken: no checksums.txt; refusing to install
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)

set "GOT="
for /f "skip=1 delims=" %%H in ('certutil -hashfile "%WORKDIR%\%ASSET%" SHA256') do if not defined GOT set "GOT=%%H"
set "GOT=!GOT: =!"
set "WANT="
for /f "usebackq tokens=1*" %%A in ("%WORKDIR%\checksums.txt") do (
  echo %%B | findstr /i /c:"%ASSET%" >nul
  if not errorlevel 1 set "WANT=%%A"
)
if not defined WANT (
  echo wheretoken: checksums.txt did not list %ASSET%
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
if /i not "!GOT!"=="!WANT!" (
  echo wheretoken: SHA256 mismatch for %ASSET%
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)

mkdir "%WORKDIR%\out" || exit /b 1
tar.exe -xf "%WORKDIR%\%ASSET%" -C "%WORKDIR%\out"
if errorlevel 1 (
  echo wheretoken: extract failed
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
if not exist "%WORKDIR%\out\wheretoken.exe" (
  echo wheretoken: archive had no wheretoken.exe
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)

mkdir "%BIN_DIR%" 2>nul
copy /Y "%WORKDIR%\out\wheretoken.exe" "%BIN_DIR%\wheretoken.exe" >nul
if errorlevel 1 (
  echo wheretoken: could not write %BIN_DIR%\wheretoken.exe
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
rmdir /s /q "%WORKDIR%" 2>nul

set "PATH=%BIN_DIR%;%PATH%"
call :AddUserPath "%BIN_DIR%"

set "EXE=%BIN_DIR%\wheretoken.exe"
set "VER="
for /f "usebackq delims=" %%V in (`"%EXE%" --version 2^>nul`) do set "VER=%%V"
if defined VER (
  echo wheretoken: installed !VER!
) else (
  echo wheretoken: installed
)
echo %EXE%
echo next: "%EXE%" update
echo next: "%EXE%" uninstall
exit /b 0

:AddUserPath
set "ADD=%~1"
set "UPATH="
for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v Path 2^>nul') do set "UPATH=%%B"
if defined UPATH (
  echo ;!UPATH!; | find /I ";%ADD%;" >nul
  if not errorlevel 1 goto :eof
  set "NEW=%ADD%;!UPATH!"
) else (
  set "NEW=%ADD%"
)
reg add "HKCU\Environment" /v Path /t REG_EXPAND_SZ /d "!NEW!" /f >nul
goto :eof
