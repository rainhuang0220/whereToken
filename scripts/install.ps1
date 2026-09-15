# Install wheretoken from GitHub Releases. No Go required.
#   irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
# Optional: $env:WHERETOKEN_VERSION = '0.1.0'; $env:PREFIX = "$env:LOCALAPPDATA\whereToken"
# Releases: https://github.com/rainhuang0220/whereToken
& {
  $ErrorActionPreference = 'Stop'
  $ProgressPreference = 'SilentlyContinue'
  try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
  } catch {
  }

  $repo = if ($env:WHERETOKEN_REPO) { $env:WHERETOKEN_REPO } else { 'rainhuang0220/whereToken' }
  $prefix = if ($env:PREFIX) { $env:PREFIX } else { Join-Path $env:LOCALAPPDATA 'whereToken' }
  $binDir = if ($env:BIN_DIR) { $env:BIN_DIR } else { Join-Path $prefix 'bin' }

  $arch = $env:PROCESSOR_ARCHITEW6432
  if ([string]::IsNullOrWhiteSpace($arch)) { $arch = $env:PROCESSOR_ARCHITECTURE }
  $goarch = $null
  switch -Regex ($arch) {
    'AMD64' { $goarch = 'amd64' }
    'ARM64' { $goarch = 'arm64' }
    default {
      throw "wheretoken: unsupported architecture $arch. whereToken ships Windows amd64 and arm64 zips.`nwheretoken: install did not copy a binary or change PATH."
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

  function Test-PathListContains([string]$pathValue, [string]$dir) {
    if ([string]::IsNullOrEmpty($pathValue) -or [string]::IsNullOrEmpty($dir)) { return $false }
    $want = $dir.TrimEnd('\')
    foreach ($part in @($pathValue -split ';' | Where-Object { $_ -ne '' })) {
      if ([string]::Equals($part.TrimEnd('\'), $want, [StringComparison]::OrdinalIgnoreCase)) {
        return $true
      }
    }
    return $false
  }

  function Add-UserPath([string]$dir) {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) { $userPath = '' }
    if (-not (Test-PathListContains $userPath $dir)) {
      $new = if ($userPath) { "$dir;$userPath" } else { $dir }
      New-ItemProperty -Path 'HKCU:\Environment' -Name Path -PropertyType ExpandString -Value $new -Force | Out-Null
      try {
        # Dummy delete broadcasts WM_SETTINGCHANGE without rewriting Path as REG_SZ.
        [Environment]::SetEnvironmentVariable('WT_PATH_REFRESH', [NullString]::Value, 'User')
      } catch {
      }
      Write-Host 'wheretoken: User PATH updated'
    }
    if (-not (Test-PathListContains $env:PATH $dir)) {
      $env:PATH = "$dir;$env:PATH"
    }
  }

  function Get-Sha256File([string]$path) {
    $sha = [System.Security.Cryptography.SHA256]::Create()
    $fs = [System.IO.File]::OpenRead($path)
    try {
      return ([BitConverter]::ToString($sha.ComputeHash($fs))).Replace('-', '').ToLowerInvariant()
    } finally {
      $fs.Dispose()
      $sha.Dispose()
    }
  }

  $asset = "wheretoken_windows_${goarch}.zip"
  $url = "$base/$asset"
  $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('wheretoken-' + [guid]::NewGuid().ToString())
  New-Item -ItemType Directory -Path $tmp | Out-Null
  try {
    $zip = Join-Path $tmp $asset
    Write-Host "wheretoken: downloading $url"
    try {
      Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing -UserAgent 'wheretoken-install'
    } catch {
      throw "wheretoken: download failed for $url : $($_.Exception.Message)`nwheretoken: install did not copy a binary or change PATH."
    }
    $sumsUrl = "$base/checksums.txt"
    $sums = Join-Path $tmp 'checksums.txt'
    try {
      Invoke-WebRequest -Uri $sumsUrl -OutFile $sums -UseBasicParsing -UserAgent 'wheretoken-install'
    } catch {
      throw "wheretoken: no checksums.txt at $sumsUrl ; refusing to install`nwheretoken: install did not copy a binary or change PATH."
    }
    $line = Get-Content $sums | Where-Object { $_ -like "*$asset*" } | Select-Object -First 1
    if (-not $line) { throw "wheretoken: checksums.txt did not list $asset`nwheretoken: install did not copy a binary or change PATH." }
    $want = ($line -split '\s+')[0].ToLower()
    $got = Get-Sha256File $zip
    if ($got -ne $want) { throw "wheretoken: SHA256 mismatch for $asset`nwheretoken: install did not copy a binary or change PATH." }
    Write-Host 'wheretoken: checksum ok'
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($zip, $tmp)
    $exe = Get-ChildItem -Path $tmp -Filter wheretoken.exe -Recurse | Select-Object -First 1
    if (-not $exe) { throw "wheretoken: archive had no wheretoken.exe`nwheretoken: install did not copy a binary or change PATH." }
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    $dest = Join-Path $binDir 'wheretoken.exe'
    $staging = Join-Path $binDir 'wheretoken.exe.new'
    Copy-Item -LiteralPath $exe.FullName -Destination $staging -Force
    Move-Item -LiteralPath $staging -Destination $dest -Force
    Write-Host "wheretoken: installed $dest"
    Add-UserPath $binDir
    $resolved = Get-Command wheretoken -CommandType Application -ErrorAction SilentlyContinue
    if (-not $resolved) {
      Write-Host "wheretoken: binary is installed, but this shell cannot resolve the command name."
      Write-Host "wheretoken: run `"$dest`" and file a bug."
      throw 'wheretoken: command not available in this shell after PATH registration'
    }
    Write-Host 'wheretoken: command available'
    & wheretoken --version
    if ($null -ne $LASTEXITCODE -and $LASTEXITCODE -ne 0) {
      throw "wheretoken: command resolved, but wheretoken --version failed (exit $LASTEXITCODE)."
    }
    Write-Host 'next: wheretoken update'
    Write-Host 'next: wheretoken uninstall'
  } finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
  }
}
