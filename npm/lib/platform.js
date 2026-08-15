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

function parseChecksum(text, name) {
  const want = String(name || '')
  if (!want) return ''
  for (const line of String(text || '').split(/\r?\n/)) {
    const parts = line.trim().split(/\s+/)
    if (parts.length < 2) continue
    const file = parts[parts.length - 1].replace(/^\*/, '')
    if (file === want || file.endsWith('/' + want)) {
      return parts[0].toLowerCase()
    }
  }
  return ''
}

module.exports = { githubAsset, parseChecksum }
