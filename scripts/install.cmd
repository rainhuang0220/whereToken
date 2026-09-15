@echo off
setlocal EnableExtensions EnableDelayedExpansion
rem cmd.exe installer. No PowerShell required.
rem   curl.exe -fsSL -o "%TEMP%\wt-install.cmd" https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.cmd && call "%TEMP%\wt-install.cmd"

if not defined WHERETOKEN_REPO set "WHERETOKEN_REPO=rainhuang0220/whereToken"
if not defined PREFIX set "PREFIX=%LOCALAPPDATA%\whereToken"
if not defined BIN_DIR set "BIN_DIR=%PREFIX%\bin"
if defined BIN_DIR if "!BIN_DIR:~-1!"=="\" set "BIN_DIR=!BIN_DIR:~0,-1!"

set "ARCH=%PROCESSOR_ARCHITECTURE%"
if defined PROCESSOR_ARCHITEW6432 if not "%PROCESSOR_ARCHITEW6432%"=="" set "ARCH=%PROCESSOR_ARCHITEW6432%"
set "GOARCH="
if /i "%ARCH%"=="AMD64" set "GOARCH=amd64"
if /i "%ARCH%"=="ARM64" set "GOARCH=arm64"
if not defined GOARCH (
  echo wheretoken: unsupported architecture %ARCH%. whereToken ships Windows amd64 and arm64 zips.
  echo wheretoken: install did not copy a binary or change PATH.
  exit /b 1
)

where curl.exe >nul 2>nul
if errorlevel 1 (
  echo wheretoken: need curl.exe on PATH
  echo wheretoken: install did not copy a binary or change PATH.
  exit /b 1
)
where tar.exe >nul 2>nul
if errorlevel 1 (
  echo wheretoken: need tar.exe on PATH
  echo wheretoken: install did not copy a binary or change PATH.
  exit /b 1
)
where certutil.exe >nul 2>nul
if errorlevel 1 (
  echo wheretoken: need certutil.exe on PATH
  echo wheretoken: install did not copy a binary or change PATH.
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
set "URL=%BASE%/%ASSET%"
set "SUMS_URL=%BASE%/checksums.txt"
set "WORKDIR=%TEMP%\wheretoken-install-%RANDOM%%RANDOM%"
mkdir "%WORKDIR%" || exit /b 1

echo wheretoken: downloading %URL%
curl.exe -fsSL -A wheretoken-install -o "%WORKDIR%\%ASSET%" "%URL%"
if errorlevel 1 (
  echo wheretoken: download failed for %URL%
  echo wheretoken: install did not copy a binary or change PATH.
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
curl.exe -fsSL -A wheretoken-install -o "%WORKDIR%\checksums.txt" "%SUMS_URL%"
if errorlevel 1 (
  echo wheretoken: no checksums.txt at %SUMS_URL%; refusing to install
  echo wheretoken: install did not copy a binary or change PATH.
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
  echo wheretoken: install did not copy a binary or change PATH.
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
if /i not "!GOT!"=="!WANT!" (
  echo wheretoken: SHA256 mismatch for %ASSET%
  echo wheretoken: install did not copy a binary or change PATH.
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
echo wheretoken: checksum ok

mkdir "%WORKDIR%\out" || exit /b 1
tar.exe -xf "%WORKDIR%\%ASSET%" -C "%WORKDIR%\out"
if errorlevel 1 (
  echo wheretoken: extract failed for %ASSET%
  echo wheretoken: install did not copy a binary or change PATH.
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
set "SRC="
if exist "%WORKDIR%\out\wheretoken.exe" set "SRC=%WORKDIR%\out\wheretoken.exe"
if not defined SRC (
  for /r "%WORKDIR%\out" %%F in (wheretoken.exe) do if not defined SRC set "SRC=%%F"
)
if not defined SRC (
  echo wheretoken: archive had no wheretoken.exe
  echo wheretoken: install did not copy a binary or change PATH.
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)

mkdir "%BIN_DIR%" 2>nul
copy /Y "%SRC%" "%BIN_DIR%\wheretoken.exe.new" >nul
if errorlevel 1 (
  echo wheretoken: could not write %BIN_DIR%\wheretoken.exe.new
  echo wheretoken: install did not change PATH.
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
move /Y "%BIN_DIR%\wheretoken.exe.new" "%BIN_DIR%\wheretoken.exe" >nul
if errorlevel 1 (
  echo wheretoken: could not replace %BIN_DIR%\wheretoken.exe
  echo wheretoken: PATH was not changed.
  del /f /q "%BIN_DIR%\wheretoken.exe.new" 2>nul
  rmdir /s /q "%WORKDIR%" 2>nul
  exit /b 1
)
rmdir /s /q "%WORKDIR%" 2>nul
echo wheretoken: installed %BIN_DIR%\wheretoken.exe

echo ;%PATH%; | find.exe /I ";%BIN_DIR%;" >nul
if errorlevel 1 set "PATH=%BIN_DIR%;%PATH%"

call :AddUserPath "%BIN_DIR%"
if errorlevel 1 (
  echo wheretoken: User PATH was not updated; this shell should still run wheretoken
)

set "EXE=%BIN_DIR%\wheretoken.exe"
set "VER="
for /f "usebackq delims=" %%V in (`"%EXE%" --version 2^>nul`) do set "VER=%%V"
if defined VER echo wheretoken: !VER!

rem Turn off delayed expansion before exporting PATH so "!" in PATH is preserved.
setlocal DisableDelayedExpansion
set "WT_PATH=%PATH%"
endlocal & endlocal & set "PATH=%WT_PATH%"

where wheretoken >nul 2>nul
if errorlevel 1 (
  echo wheretoken: binary is installed, but this shell cannot resolve the command name.
  echo wheretoken: run the printed executable path and file a bug.
  exit /b 1
)
echo wheretoken: command available
wheretoken --version
if errorlevel 1 (
  echo wheretoken: command resolved, but wheretoken --version failed.
  exit /b 1
)
echo next: wheretoken update
echo next: wheretoken uninstall
exit /b 0

:AddUserPath
set "ADD=%~1"
if "!ADD:~-1!"=="\" set "ADD=!ADD:~0,-1!"
set "UPATH="
for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v Path 2^>nul') do set "UPATH=%%B"
if defined UPATH (
  echo ;!UPATH!; | find.exe /I ";%ADD%;" >nul
  if not errorlevel 1 goto :BroadcastEnv
  set "NEW=%ADD%;!UPATH!"
) else (
  set "NEW=%ADD%"
)
reg add "HKCU\Environment" /v Path /t REG_EXPAND_SZ /d "!NEW!" /f >nul
if errorlevel 1 (
  echo wheretoken: failed to write HKCU\Environment Path
  exit /b 1
)
echo wheretoken: User PATH updated
goto :BroadcastEnv

:BroadcastEnv
where powershell.exe >nul 2>nul
if errorlevel 1 exit /b 0
powershell.exe -NoProfile -NonInteractive -Command "[Environment]::SetEnvironmentVariable('WT_PATH_REFRESH',[NullString]::Value,'User')" >nul 2>nul
exit /b 0
