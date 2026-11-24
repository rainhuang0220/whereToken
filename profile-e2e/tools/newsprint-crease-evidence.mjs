import { chromium } from '@playwright/test';
import assert from 'node:assert/strict';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { once } from 'node:events';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = fileURLToPath(new URL('../../', import.meta.url));
const bundle = path.resolve(process.env.WT_PROFILE_STATIC || path.join(repo, 'internal/profilewebembed/static'));
const fixture = path.join(repo, 'public-profile/profile.json');
const serveOnly = process.argv.includes('--serve');
const output = serveOnly ? null : path.resolve(process.argv[2]);
const mount = '/whereToken/profile/';
const key = 'wheretoken.newsprint.creases.v1';
const mime = { '.css': 'text/css', '.jpg': 'image/jpeg', '.js': 'text/javascript',
  '.svg': 'image/svg+xml', '.png': 'image/png', '.json': 'application/json', '.html': 'text/html' };
const server = http.createServer(async (req, res) => {
  const pathname = new URL(req.url, 'http://localhost').pathname;
  if (!pathname.startsWith(mount)) return res.writeHead(404).end();
  const relative = pathname.slice(mount.length) || 'index.html';
  const file = path.resolve(bundle, relative);
  if (!file.startsWith(bundle + path.sep)) return res.writeHead(404).end();
  try {
    res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
    res.end(await readFile(relative === 'profile.json' ? fixture : file));
  } catch {
    res.writeHead(404).end();
  }
});

server.listen(Number(process.env.WT_PREVIEW_PORT || 0), '127.0.0.1');
await once(server, 'listening');
const url = `http://127.0.0.1:${server.address().port}${mount}?palette=newsprint`;
if (serveOnly) {
  console.log(url);
} else {
  await mkdir(output, { recursive: true });
  const browser = await chromium.launch();
  try {
    if (process.argv.includes('--fold')) {
      await exerciseFold(browser);
    } else if (process.argv.includes('--stock')) {
      await verifyStockPhase(browser);
    } else {
      const cases = process.env.WT_SINGLE ? [[1440, 900, 'light', 1]] : [
        [1440, 900, 'light', 1], [1440, 900, 'dark', 2],
        [1920, 1080, 'light', 2], [1920, 1080, 'dark', 1],
      ];
      const results = [];
      for (const args of cases) results.push(await capture(browser, ...args));
      await writeFile(path.join(output, 'visual.json'), JSON.stringify(results, null, 2) + '\n');
      console.log(results.map(({ name, pixels }) => ({ name, pixels })));
      if (!process.env.WT_OBSERVE_ONLY) {
        for (const { name, pixels } of results) {
          assert.ok(pixels.paperChanged > 10, `${name}: the physical scar must cross the crop`);
          assert.ok(pixels.inkChanged > 20 && pixels.maxInkBrightening <= 20 && pixels.maxInkDelta <= 45,
            `${name}: normal lighting must reach ink without a white streak`);
          assert.ok(pixels.outsideChanged <= 10, `${name}: lighting must stay local to the scar`);
        }
      }
    }
  } finally {
    await browser.close();
    server.close();
  }
}

async function ready(page, revision = -1) {
  await page.waitForFunction(previous => {
    const paper = window.__WT_PAPER3D__?.getDiagnostics();
    return (paper?.snapshotReady && paper.readyRevision > previous) || paper?.state === 'degraded';
  }, revision, { timeout: 30000 });
  const paper = await page.evaluate(() => window.__WT_PAPER3D__.getDiagnostics());
  assert.ok(paper.snapshotReady, paper.error);
  return paper;
}

async function capture(browser, width, height, colorScheme, deviceScaleFactor) {
  const name = `${width}-${colorScheme}-dpr${deviceScaleFactor}`;
  const page = await browser.newPage({ viewport: { width, height }, colorScheme, deviceScaleFactor });
  await page.goto(url + '&range=53w');
  await page.locator('#hero-value').evaluate(async node => {
    // Chromium can resolve the platform UI-monospace alias differently
    // between navigations. Pin the glyph face in this pixel experiment.
    node.style.fontFamily = 'Menlo, monospace';
    await document.fonts.ready;
  });
  const flat = await ready(page);
  // A calibrated v1 line isolates the same opaque glyphs in both builds.
  // The production worker restores this crease and computes its real mesh.
  const { clip, line, color, saved } = await page.locator('#hero-value').evaluate((node, { width, height }) => {
    const range = document.createRange();
    range.selectNodeContents(node);
    const rect = range.getBoundingClientRect();
    const x = Math.round(rect.left + rect.width / 2);
    const y = Math.round(rect.top + rect.height * .55);
    const nx = .23, ny = Math.sqrt(1 - nx ** 2);
    const origin = { x: -width / 200, y: height / 200 };
    const depth = (y * ny - x * nx) / 100;
    const start = { x: origin.x, y: origin.y - depth / ny };
    const end = { x: origin.x + (height * ny / 100 - depth) / nx, y: -origin.y };
    if (end.x > width / 200) {
      end.x = width / 200;
      end.y = origin.y - (depth + width * nx / 100) / ny;
    }
    const length = Math.hypot(end.x - start.x, end.y - start.y);
    return {
      clip: { x: Math.round(rect.left) - 12, y: Math.round(rect.top) - 5,
        width: Math.ceil(rect.width) + 24, height: Math.ceil(rect.height) + 10 },
      line: { x, y, slope: nx / ny },
      color: getComputedStyle(node).color.match(/\d+/g).slice(0, 3).map(Number),
      saved: { width: width / 100, height: height / 100, memories: [{
        strength: .72, band: .2, times: 1,
        hinge: { kind: 'corner', nx, ny, depth, origin, start, end, length, span: length, fullSpan: true, alongShift: 0 },
      }] },
    };
  }, { width: flat.sheetCssWidth, height: flat.sheetCssHeight });
  const capture = (phase, extra = {}) => page.screenshot({ clip, path: path.join(output, name + '-' + phase + '.png'), ...extra });
  const position = () => page.locator('#hero-value').evaluate(node => {
    const range = document.createRange();
    range.selectNodeContents(node);
    return [range.getBoundingClientRect().toJSON(), getComputedStyle(node).font, scrollY];
  });
  const flatPosition = await position();
  const before = await capture('flat-ink');
  const hero = page.locator('#hero-value');
  await hero.evaluate(node => { node.style.visibility = 'hidden'; });
  const bare = await capture('flat-paper');
  await hero.evaluate(node => { node.style.removeProperty('visibility'); });
  await page.screenshot({ fullPage: true, path: path.join(output, name + '-flat-full.png') });
  await page.evaluate(({ key, saved }) => sessionStorage.setItem(key, JSON.stringify(saved)), { key, saved });
  await page.reload();
  await page.locator('#hero-value').evaluate(async node => {
    // Chromium can resolve the platform UI-monospace alias differently
    // between navigations. Pin the glyph face in this pixel experiment.
    node.style.fontFamily = 'Menlo, monospace';
    await document.fonts.ready;
  });
  const scar = await ready(page);
  assert.equal(scar.plasticCreases, 1);
  const scarPosition = await position();
  assert.deepEqual(scarPosition, flatPosition, 'identical native glyph geometry');
  const after = await capture('scar-ink');
  await page.screenshot({ fullPage: true, path: path.join(output, name + '-scar-full.png') });
  const pixels = await page.evaluate(async ({ shots, clip, line, color }) => {
    const imgs = await Promise.all(shots.map(async encoded => {
      const bitmap = await createImageBitmap(new Blob([Uint8Array.from(atob(encoded), char => char.charCodeAt(0))], { type: 'image/png' }));
      const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
      const ctx = canvas.getContext('2d');
      ctx.drawImage(bitmap, 0, 0);
      bitmap.close();
      return ctx.getImageData(0, 0, canvas.width, canvas.height);
    }));
    const [flat, bare, scar] = imgs;
    const pixel = (img, i) => [img.data[i], img.data[i + 1], img.data[i + 2]];
    const delta = (a, b) => Math.max(...a.map((v, c) => Math.abs(v - b[c])));
    const stats = { inkChanged: 0, paperChanged: 0, outsideChanged: 0, inkSamples: 0, maxInkDelta: 0, maxInkBrightening: 0, maxInkChannelBrightening: 0, maxPaperDelta: 0 };
    for (let y = 0; y < flat.height; y++) for (let x = 0; x < flat.width; x++) {
      const cssX = clip.x + (x + .5) / devicePixelRatio;
      const cssY = clip.y + (y + .5) / devicePixelRatio;
      const distance = Math.abs(cssY - line.y - (cssX - line.x) * line.slope);
      const i = (y * flat.width + x) * 4;
      const f = pixel(flat, i), b = pixel(bare, i), s = pixel(scar, i);
      const change = delta(f, s);
      if (distance > 25 && change > 4) stats.outsideChanged++;
      if (distance > 14) continue;
      if (delta(f, color) < 4 && delta(f, b) > 45) {
        stats.inkSamples++;
        stats.inkChanged += Number(change > 0);
        stats.maxInkDelta = Math.max(stats.maxInkDelta, change);
        const [red, green, blue] = s.map((v, c) => v - f[c]);
        const lumaChange = .2126 * red + .7152 * green + .0722 * blue;
        stats.maxInkBrightening = Math.max(stats.maxInkBrightening, lumaChange);
        stats.maxInkChannelBrightening = Math.max(stats.maxInkChannelBrightening, red, green, blue);
      } else if (delta(f, b) < 2) {
        stats.paperChanged += Number(change > 2);
        stats.maxPaperDelta = Math.max(stats.maxPaperDelta, change);
      }
    }
    return stats;
  }, { shots: [before, bare, after].map(buf => buf.toString('base64')), clip, line, color });
  await page.close();
  return { name, clip, line, color, flatPosition, scarPosition, pixels, flat, scar };
}


// An actual drag, a crease that crosses the headline, and fifty real
// control clicks. The v1 comparison projects the renderer's stored line
// with the same independent-axis transformation used by 2263064.
async function exerciseFold(browser) {
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, colorScheme: 'light' });
  const screenshot = (name, options = {}) => page.screenshot({ path: path.join(output, name + '.png'), ...options });
  const memory = () => page.evaluate(key => JSON.parse(sessionStorage.getItem(key)), key);
  const tab = name => page.locator('#ranges').getByRole('tab', { name, exact: true });
  await page.goto(url + '&range=53w');
  let d = await ready(page);
  await page.locator('#hero-value').evaluate(node => { node.style.fontFamily = 'Menlo, monospace'; });
  const clip = await page.locator('#hero-value').evaluate(node => {
    const range = document.createRange();
    range.selectNodeContents(node);
    const r = range.getBoundingClientRect();
    return { x: Math.floor(r.x - 12), y: Math.floor(r.y - 5), width: Math.ceil(r.width + 24), height: Math.ceil(r.height + 10) };
  });
  await screenshot('gesture-flat-local', { clip });
  await screenshot('gesture-flat-full', { fullPage: true });
  const x = 12, y = 13;
  await page.mouse.move(x, y);
  await page.mouse.down();
  for (let step = 1; step <= 40; step++) {
    await page.mouse.move(x + step / 40 * 226, y + step / 40 * 226);
    await new Promise(resolve => setTimeout(resolve, 23));
  }
  await page.waitForFunction(() => window.__WT_PAPER3D__.getDiagnostics().angleDegrees > 100);
  await screenshot('gesture-bending');
  await page.mouse.up();
  await page.waitForFunction(() => {
    const d = window.__WT_PAPER3D__.getDiagnostics();
    return ['resting', 'latched'].includes(d.phase) && d.plasticCreases === 1;
  });
  const released = await ready(page);
  if (released.phase === 'latched') {
    const tip = released.flapPoints.find(point => point.u < .5 && point.v > .5);
    assert.ok(tip);
    await screenshot('gesture-latched');
    await page.mouse.move(tip.x, tip.y);
    await page.mouse.down();
    for (let step = 1; step <= 28; step++) {
      await page.mouse.move(tip.x - step / 28 * 205, tip.y - step / 28 * 205);
      await new Promise(resolve => setTimeout(resolve, 20));
    }
    await page.waitForFunction(() => window.__WT_PAPER3D__.getDiagnostics().targetDegrees < 100);
    await page.mouse.up();
    await page.waitForFunction(() => window.__WT_PAPER3D__.getDiagnostics().phase === 'resting');
  }
  await page.mouse.move(500, 350);
  await screenshot('gesture-released-local', { clip });
  await screenshot('gesture-released-full', { fullPage: true });
  await page.mouse.move(clip.x + 13, clip.y + clip.height * .6);
  await page.mouse.down();
  await page.mouse.move(clip.x + clip.width - 13, clip.y + clip.height * .6, { steps: 12 });
  await page.mouse.up();
  const nativeSelection = await page.evaluate(() => document.getSelection()?.toString());
  assert.match(nativeSelection, /[0-9].*[0-9]/);
  await page.evaluate(() => document.getSelection()?.removeAllRanges());
  d = await ready(page);
  const stored = await memory();
  assert.ok(stored.memories.length === 1);
  const position = diagnostic => {
    if (diagnostic.creasePositions) return diagnostic.creasePositions[0];
    const { hinge } = stored.memories[0];
    const point = ({ x, y }) => ({
      x: (x / stored.width + .5) * diagnostic.sheetCssWidth,
      y: (.5 - y / stored.height) * diagnostic.sheetCssHeight,
    });
    return { origin: point(hinge.origin), start: point(hinge.start), end: point(hinge.end) };
  };
  const sample = (label, diagnostic, round) => ({
    label, round, height: diagnostic.sheetCssHeight, crease: position(diagnostic),
    stockReused: diagnostic.stockReused, inkReused: diagnostic.inkReused,
    lightReused: diagnostic.lightReused, creaseLighting: diagnostic.creaseLighting,
    stockCacheBytes: diagnostic.stockCacheBytes, lightCacheBytes: diagnostic.lightCacheBytes,
    inkCacheBytes: diagnostic.inkCacheBytes, geometryMs: diagnostic.geometryMs,
    captureMs: diagnostic.captureMs, drawMs: diagnostic.drawMs, bitmapMs: diagnostic.bitmapMs,
  });
  const samples = [sample('53w', d, 0)];
  await page.evaluate(() => {
    window.creaseFrames = [];
    function observe(now) {
      const d = window.__WT_PAPER3D__.getDiagnostics();
      window.creaseFrames.push({ now, revision: d.captureRevision, ready: d.snapshotReady,
        light: d.creaseLighting, title: document.querySelector('#hero-value').textContent,
        range: document.querySelector('#ranges [aria-selected=true]').textContent });
      if (!d.snapshotReady) requestAnimationFrame(observe);
    }
    // Registered after the controller, so the first sample observes
    // the backdrop it has prepared for this rendering opportunity.
    window.addEventListener('wt:profile-render', () => requestAnimationFrame(observe));
  });
  const expected = position(d);
  async function change(label, round) {
    const start = performance.now();
    await tab(label).click();
    assert.equal(await tab(label).getAttribute('aria-selected'), 'true');
    d = await ready(page, d.readyRevision);
    samples.push({ ...sample(label, d, round), readyMs: Math.round(performance.now() - start) });
  }
  for (let round = 1; round <= 10; round++) {
    for (const label of ['All', '53w', '7d', '30d', '53w']) {
      await change(label, round);
      if (round === 1) await screenshot('range-' + label, { fullPage: true });
    }
  }
  const topError = Math.max(...samples.flatMap(({ crease }) =>
    ['start', 'end'].map(end => Math.hypot(crease[end].x - expected[end].x, crease[end].y - expected[end].y))));
  const frames = await page.evaluate(() => window.creaseFrames);
  const warmFrames = frames.filter(frame => frame.revision > d.captureRevision - 40);
  const warmMissing = warmFrames.filter(frame => !frame.light).length;
  const rapidFrom = d.readyRevision;
  for (const label of ['All', '7d', '30d', '53w']) await tab(label).evaluate(node => node.click());
  d = await ready(page, rapidFrom);
  assert.match(d.documentKey, /range=53w/);
  await page.evaluate(() => scrollTo(0, 520));
  await page.waitForFunction(() => scrollY >= 500);
  assert.deepEqual(position(await ready(page)), position(d));
  await page.evaluate(() => scrollTo(0, 0));

  const extras = {};
  const details = page.locator('#coverage summary');
  await details.click();
  d = await ready(page, d.readyRevision);
  extras.detailsOpen = sample('Technical details', d);
  await details.click();
  d = await ready(page, d.readyRevision);
  await page.evaluate(() => scrollTo(0, 0));
  if (!process.env.WT_OBSERVE_ONLY) {
    const cdp = await page.context().newCDPSession(page);
    await cdp.send('Emulation.setDeviceMetricsOverride', {
      width: 1280, height: 900, deviceScaleFactor: 1.25, mobile: false,
    });
    d = await ready(page, d.readyRevision);
    extras.fractional = { ...sample('1280 DPR 1.25', d), scale: d.effectiveInkScale };
    await cdp.send('Emulation.clearDeviceMetricsOverride');
    d = await ready(page, d.readyRevision);
    await cdp.detach();
    for (const { crease } of Object.values(extras)) {
      for (const end of ['start', 'end']) {
        assert.ok(Math.hypot(crease[end].x - expected[end].x, crease[end].y - expected[end].y) <= 2);
      }
    }
  }
  assert.deepEqual(await memory(), stored);
  await page.reload();
  d = await ready(page);
  assert.deepEqual(position(d), expected);
  await page.getByRole('tab', { name: 'Cobalt', exact: true }).click();
  assert.equal(await memory(), null);
  await page.getByRole('tab', { name: 'Newsprint', exact: true }).click();
  d = await ready(page);
  assert.equal(d.plasticCreases, 0);
  await writeFile(path.join(output, 'coordinates.json'), JSON.stringify({ samples, topError, warmMissing, frames, extras, nativeSelection }, null, 2) + '\n');
  console.log(`Max endpoint error after ten rounds: ${topError.toFixed(3)} CSS px`);
  if (!process.env.WT_OBSERVE_ONLY) {
    assert.ok(topError <= 2);
    assert.ok(samples.every(s => s.creaseLighting));
    assert.equal(warmMissing, 0, 'an exact cached crease remains visible throughout warm switches');
  }
  await page.close();
}


async function verifyStockPhase(browser) {
  const results = [];
  for (const [width, deviceScaleFactor, colorScheme] of [[1440, 1, 'light'], [1440, 2, 'dark'], [1920, 2, 'light']]) {
    const page = await browser.newPage({ viewport: { width, height: 900 }, deviceScaleFactor, colorScheme });
    const readStock = () => page.locator('#wt-paper-resident').evaluate(source => {
      const canvas = new OffscreenCanvas(384, 384);
      const ctx = canvas.getContext('2d');
      ctx.drawImage(source, 80, 120, 384, 384, 0, 0, 384, 384);
      return Array.from(ctx.getImageData(0, 0, 384, 384).data);
    });
    await page.goto(url + '&range=53w');
    const tall = await ready(page);
    const tallPixels = await readStock();
    await page.locator('#ranges').getByRole('tab', { name: '7d', exact: true }).click();
    const short = await ready(page, tall.readyRevision);
    const shortPixels = await readStock();
    await page.reload();
    const direct = await ready(page);
    const directPixels = await readStock();
    assert.ok(tall.sheetCssHeight > short.sheetCssHeight);
    assert.equal(direct.stockCropped, false);
    assert.equal(short.stockCropped, Number.isInteger(tall.effectiveInkScale));
    const delta = (a, b) => a.reduce((max, v, i) => Math.max(max, Math.abs(v - b[i])), 0);
    const directDelta = delta(shortPixels, directPixels);
    assert.ok(directDelta <= 1, 'stock crop matches the independently rendered shorter paper');
    const tallDelta = delta(shortPixels, tallPixels);
    if (short.stockCropped) assert.equal(tallDelta, 0, 'the integer-scale crop starts at the same top-left texel');
    results.push({ width, deviceScaleFactor, colorScheme, tallScale: tall.effectiveInkScale,
      shortScale: short.effectiveInkScale, cropped: short.stockCropped, directDelta, tallDelta });
    await page.close();
  }
  await writeFile(path.join(output, 'stock-phase.json'), JSON.stringify(results, null, 2) + '\n');
  console.log(results);
}
