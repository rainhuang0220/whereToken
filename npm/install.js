#!/usr/bin/env node
'use strict'

const fs = require('fs')
const http = require('https')
const os = require('os')
const path = require('path')
const crypto = require('crypto')
const { githubAsset, parseChecksum } = require('./lib/platform')

const pkg = require('./package.json')
const destDir = path.join(__dirname, 'bin')

function log(msg) {
  process.stderr.write(`wheretoken: ${msg}\n`)
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const req = http.get(url, { headers: { 'User-Agent': 'wheretoken-npm' } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume()
        download(res.headers.location, dest).then(resolve, reject)
        return
      }
      if (res.statusCode !== 200) {
        const err = new Error(`GET ${url} → ${res.statusCode}`)
        err.statusCode = res.statusCode
        res.resume()
        reject(err)
        return
      }
      const out = fs.createWriteStream(dest)
      res.pipe(out)
      out.on('finish', () => out.close(resolve))
      out.on('error', reject)
    })
    req.on('error', reject)
  })
}

function extract(archive, ext, binaryName) {
  fs.mkdirSync(destDir, { recursive: true })
  const target = path.join(destDir, binaryName)
  if (ext === 'zip') {
    const r = spawnSync('tar', ['-xf', archive, '-C', destDir], { stdio: 'inherit' })
    if (r.status !== 0) {
      throw new Error('failed to unzip (need tar, which Windows 10+ ships)')
    }
  } else {
    const r = spawnSync('tar', ['-xzf', archive, '-C', destDir], { stdio: 'inherit' })
    if (r.status !== 0) {
      throw new Error('failed to untar')
    }
  }
  if (!fs.existsSync(target)) {
    // goreleaser may nest or flatten; search
    const found = walkFind(destDir, binaryName)
    if (found && found !== target) {
      fs.renameSync(found, target)
    }
  }
  if (!fs.existsSync(target)) {
    throw new Error(`archive did not contain ${binaryName}`)
  }
  fs.chmodSync(target, 0o755)
}

function walkFind(dir, name) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name)
    if (ent.isDirectory()) {
      const hit = walkFind(p, name)
      if (hit) return hit
    } else if (ent.name === name) {
      return p
    }
  }
  return null
}

async function main() {
  if (process.env.WHERETOKEN_SKIP_DOWNLOAD === '1') {
    log('WHERETOKEN_SKIP_DOWNLOAD=1, skipping binary download')
    return
  }
  const asset = githubAsset({
    version: pkg.version,
    platform: process.platform,
    arch: process.arch,
  })
  if (asset.error) {
    log(asset.error)
    log('install with: go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest')
    return
  }
  const tmp = path.join(os.tmpdir(), asset.name)
  const sumsPath = path.join(os.tmpdir(), `wheretoken-${pkg.version}-checksums.txt`)
  try {
    await download(asset.url, tmp)
    const sumsUrl = asset.url.replace(/\/[^/]+$/, '/checksums.txt')
    await download(sumsUrl, sumsPath)
    const want = parseChecksum(fs.readFileSync(sumsPath, 'utf8'), asset.name)
    if (!want) {
      throw new Error(`checksums.txt did not list ${asset.name}`)
    }
    const got = crypto.createHash('sha256').update(fs.readFileSync(tmp)).digest('hex')
    if (got !== want) {
      throw new Error(`SHA256 mismatch for ${asset.name}`)
    }
    extract(tmp, asset.ext, asset.binary)
  } catch (err) {
    if (err.statusCode === 404) {
      log(`no GitHub release asset yet (${asset.name}).`)
      log('install with: go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest')
      return
    }
    log(err.message || String(err))
    log('install with: go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest')
    process.exitCode = 1
  } finally {
    try {
      fs.unlinkSync(tmp)
    } catch {
      // ignore
    }
    try {
      fs.unlinkSync(sumsPath)
    } catch {
      // ignore
    }
  }
}

main()
