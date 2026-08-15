# Install wheretoken from GitHub Releases. No Go required.
#   irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
# Optional: $env:WHERETOKEN_VERSION = '0.1.0'; $env:PREFIX = "$env:LOCALAPPDATA\whereToken"
$ErrorActionPreference = 'Stop'

$repo = if ($env:WHERETOKEN_REPO) { $env:WHERETOKEN_REPO } else { 'rainhuang0220/whereToken' }
$prefix = if ($env:PREFIX) { $env:PREFIX } else { Join-Path $env:LOCALAPPDATA 'whereToken' }
$binDir = Join-Path $prefix 'bin'

switch -Regex ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { $goarch = 'amd64' }
  'ARM64' { $goarch = 'arm64' }
  default {
    Write-Error "wheretoken: unsupported arch $($env:PROCESSOR_ARCHITECTURE)"
  }
}

$version = $env:WHERETOKEN_VERSION
if (-not $version) {
  try {
    $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ 'User-Agent' = 'wheretoken-install' }
    $version = [string]$rel.tag_name
  } catch {
    $version = ''
  }
}
$version = $version -replace '^v', ''

if (-not $version) {
  Write-Host 'wheretoken: no GitHub Release yet. Install with Go or npm:'
  Write-Host '  go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest'
  Write-Host '  npm install -g wheretoken'
  exit 1
}

$asset = "wheretoken_windows_${goarch}.zip"
$url = "https://github.com/$repo/releases/download/v$version/$asset"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('wheretoken-' + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  $zip = Join-Path $tmp $asset
  Write-Host "wheretoken: downloading $url"
  Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
  Expand-Archive -Path $zip -DestinationPath $tmp -Force
  $exe = Get-ChildItem -Path $tmp -Filter wheretoken.exe -Recurse | Select-Object -First 1
  if (-not $exe) { throw 'archive had no wheretoken.exe' }
  New-Item -ItemType Directory -Path $binDir -Force | Out-Null
  Copy-Item $exe.FullName (Join-Path $binDir 'wheretoken.exe') -Force
  Write-Host "wheretoken: installed $(Join-Path $binDir 'wheretoken.exe')"
  $onPath = $env:PATH -split ';' | Where-Object { $_ -eq $binDir }
  if (-not $onPath) {
    Write-Host "wheretoken: add $binDir to PATH"
  }
  & (Join-Path $binDir 'wheretoken.exe') --version
} catch {
  Write-Host "wheretoken: $($_.Exception.Message)"
  Write-Host '  go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest'
  Write-Host '  npm install -g wheretoken'
  exit 1
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
