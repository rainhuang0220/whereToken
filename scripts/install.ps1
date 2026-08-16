# Install wheretoken from GitHub Releases. No Go required.
#   irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
# Optional: $env:WHERETOKEN_VERSION = '0.1.0'; $env:PREFIX = "$env:LOCALAPPDATA\whereToken"
$ErrorActionPreference = 'Stop'

$repo = if ($env:WHERETOKEN_REPO) { $env:WHERETOKEN_REPO } else { 'rainhuang0220/whereToken' }
$prefix = if ($env:PREFIX) { $env:PREFIX } else { Join-Path $env:LOCALAPPDATA 'whereToken' }
$binDir = if ($env:BIN_DIR) { $env:BIN_DIR } else { Join-Path $prefix 'bin' }

switch -Regex ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { $goarch = 'amd64' }
  'ARM64' { $goarch = 'arm64' }
  default {
    Write-Error "wheretoken: unsupported arch $($env:PROCESSOR_ARCHITECTURE)"
  }
}

$version = $env:WHERETOKEN_VERSION
if ($version) { $version = $version -replace '^v', '' }

if ($env:WHERETOKEN_RELEASE_URL) {
  $base = $env:WHERETOKEN_RELEASE_URL.TrimEnd('/')
} elseif ($version) {
  $base = "https://github.com/$repo/releases/download/v$version"
} else {
  $base = "https://github.com/$repo/releases/latest/download"
}

function Add-UserPath([string]$dir) {
  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (-not $userPath) { $userPath = '' }
  $parts = @($userPath -split ';' | Where-Object { $_ -ne '' })
  if ($parts -contains $dir) {
    $env:PATH = "$dir;$env:PATH"
    return
  }
  $new = if ($userPath) { "$dir;$userPath" } else { $dir }
  [Environment]::SetEnvironmentVariable('Path', $new, 'User')
  $env:PATH = "$dir;$env:PATH"
}

function Show-Next {
  $exe = Join-Path $binDir 'wheretoken.exe'
  Write-Host "wheretoken: installed $exe"
  Add-UserPath $binDir
  Write-Host 'next: wheretoken'
  if (Test-Path $exe) { & $exe --version }
}

function Install-WithGo {
  if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host 'wheretoken: download failed'
    exit 1
  }
  Write-Host 'wheretoken: no GitHub Release; installing with go install'
  New-Item -ItemType Directory -Path $binDir -Force | Out-Null
  $oldGobin = $env:GOBIN
  $env:GOBIN = $binDir
  try {
    go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
  } finally {
    if ($null -eq $oldGobin) { Remove-Item Env:GOBIN -ErrorAction SilentlyContinue } else { $env:GOBIN = $oldGobin }
  }
  Show-Next
  exit 0
}

$asset = "wheretoken_windows_${goarch}.zip"
$url = "$base/$asset"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('wheretoken-' + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  $zip = Join-Path $tmp $asset
  Write-Host "wheretoken: downloading $url"
  try {
    Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
  } catch {
    if ($env:WHERETOKEN_RELEASE_URL) {
      Write-Host 'wheretoken: download failed'
      exit 1
    }
    Install-WithGo
  }
  $sumsUrl = "$base/checksums.txt"
  $sums = Join-Path $tmp 'checksums.txt'
  try {
    Invoke-WebRequest -Uri $sumsUrl -OutFile $sums -UseBasicParsing
  } catch {
    throw 'no checksums.txt; refusing to install'
  }
  $line = Get-Content $sums | Where-Object { $_ -like "*$asset*" } | Select-Object -First 1
  if (-not $line) { throw "checksums.txt did not list $asset" }
  $want = ($line -split '\s+')[0].ToLower()
  $got = (Get-FileHash -Path $zip -Algorithm SHA256).Hash.ToLower()
  if ($got -ne $want) { throw "SHA256 mismatch for $asset" }
  Expand-Archive -Path $zip -DestinationPath $tmp -Force
  $exe = Get-ChildItem -Path $tmp -Filter wheretoken.exe -Recurse | Select-Object -First 1
  if (-not $exe) { throw 'archive had no wheretoken.exe' }
  New-Item -ItemType Directory -Path $binDir -Force | Out-Null
  Copy-Item $exe.FullName (Join-Path $binDir 'wheretoken.exe') -Force
  Show-Next
} catch {
  Write-Host "wheretoken: $($_.Exception.Message)"
  exit 1
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
