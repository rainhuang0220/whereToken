const test = require('node:test')
const assert = require('node:assert/strict')
const { githubAsset, parseChecksum } = require('./platform')

test('darwin arm64 matches goreleaser archive', () => {
  const a = githubAsset({ version: '0.1.0', platform: 'darwin', arch: 'arm64' })
  assert.equal(a.name, 'wheretoken_darwin_arm64.tar.gz')
  assert.equal(a.binary, 'wheretoken')
  assert.equal(
    a.url,
    'https://github.com/rainhuang0220/whereToken/releases/download/v0.1.0/wheretoken_darwin_arm64.tar.gz',
  )
})

test('windows x64 uses zip and .exe', () => {
  const a = githubAsset({ version: 'v1.2.3', platform: 'win32', arch: 'x64' })
  assert.equal(a.name, 'wheretoken_windows_amd64.zip')
  assert.equal(a.binary, 'wheretoken.exe')
  assert.match(a.url, /\/v1\.2\.3\/wheretoken_windows_amd64\.zip$/)
})

test('linux amd64', () => {
  const a = githubAsset({ version: '0.1.0', platform: 'linux', arch: 'x64' })
  assert.equal(a.name, 'wheretoken_linux_amd64.tar.gz')
})

test('strips leading v once', () => {
  const a = githubAsset({ version: 'v0.1.0', platform: 'linux', arch: 'arm64' })
  assert.match(a.url, /\/v0\.1\.0\//)
  assert.ok(!a.url.includes('/vv'))
})

test('rejects unsupported arch', () => {
  const a = githubAsset({ version: '0.1.0', platform: 'darwin', arch: 'ia32' })
  assert.ok(a.error)
})

test('parseChecksum reads goreleaser checksums.txt', () => {
  const text = [
    'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  wheretoken_darwin_arm64.tar.gz',
    'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  wheretoken_windows_amd64.zip',
  ].join('\n')
  assert.equal(
    parseChecksum(text, 'wheretoken_windows_amd64.zip'),
    'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  )
  assert.equal(parseChecksum(text, 'missing.bin'), '')
})
