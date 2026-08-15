'use strict'

function githubAsset({ version, platform, arch }) {
  const ver = String(version || '').replace(/^v/, '')
  const os = { darwin: 'darwin', linux: 'linux', win32: 'windows' }[platform]
  const goarch = { x64: 'amd64', arm64: 'arm64' }[arch]
  if (!os || !goarch) {
    return { error: `unsupported platform ${platform}/${arch}` }
  }
  const ext = os === 'windows' ? 'zip' : 'tar.gz'
  const name = `wheretoken_${os}_${goarch}.${ext}`
  return {
    os,
    goarch,
    ext,
    name,
    binary: os === 'windows' ? 'wheretoken.exe' : 'wheretoken',
    url: `https://github.com/rainhuang0220/whereToken/releases/download/v${ver}/${name}`,
  }
}

module.exports = { githubAsset }
