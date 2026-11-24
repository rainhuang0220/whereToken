import { chromium } from '@playwright/test';
import { createWriteStream } from 'node:fs';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { once } from 'node:events';
import http from 'node:http';
import path from 'node:path';
import { createGzip } from 'node:zlib';
import { fileURLToPath } from 'node:url';

const repo = fileURLToPath(new URL('../../', import.meta.url));
const output = path.resolve(process.argv[2]);
const bundle = process.env.WT_PROFILE_STATIC || path.join(repo, 'internal/profilewebembed/static');
const fixture = path.join(repo, 'public-profile/profile.json');
const mount = '/whereToken/profile/';
const mime = { '.css': 'text/css', '.jpg': 'image/jpeg', '.js': 'text/javascript',
  '.svg': 'image/svg+xml', '.png': 'image/png', '.json': 'application/json', '.html': 'text/html' };
const server = http.createServer(async (req, res) => {
  const relative = new URL(req.url, 'http://localhost').pathname.slice(mount.length) || 'index.html';
  const file = path.resolve(bundle, relative);
  if (!file.startsWith(bundle + path.sep)) return res.writeHead(404).end();
  try {
    res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
    res.end(await readFile(relative === 'profile.json' ? fixture : file));
  } catch {
    res.writeHead(404).end();
  }
});
await mkdir(output, { recursive: true });
server.listen(0, '127.0.0.1');
await once(server, 'listening');
let browser;
const url = `http://127.0.0.1:${server.address().port}${mount}?palette=newsprint`;

async function setup(page) {
  await page.addInitScript(() => {
    const entries = window.paperPerf = { tasks: [], events: [], clicks: [], frames: [] };
    for (const [type, target] of [['longtask', 'tasks'], ['event', 'events']]) {
      new PerformanceObserver(list => {
        for (const e of list.getEntries()) entries[target].push({
          name: e.name, start: e.startTime, duration: e.duration, processingStart: e.processingStart,
          processingEnd: e.processingEnd, interactionId: e.interactionId,
        });
      }).observe({ type, buffered: true, durationThreshold: 16 });
    }
    document.addEventListener('click', event => {
      const tab = event.target.closest('[role=tab]');
      if (!tab) return;
      const click = { label: tab.textContent, start: event.timeStamp, dispatch: performance.now() };
      entries.clicks.push(click);
      requestAnimationFrame(() => {
        click.frame = performance.now();
        click.selected = document.querySelector('#ranges [aria-selected=true]')?.textContent;
      });
    }, true);
    let visibleSince = -1;
    const sample = time => {
      const diagnostic = window.__WT_PAPER3D__?.getDiagnostics();
      const physical = document.documentElement.classList.contains('wt-physical-print');
      const folded = document.documentElement.classList.contains('wt-paper-folding');
      const selected = document.querySelector('#ranges [aria-selected=true]')?.textContent;
      const hero = document.querySelector('#hero-value');
      if (hero?.textContent && hero.textContent !== '—' && visibleSince < 0) visibleSince = time;
      if (time < 1500 || time - entries.clicks.at(-1)?.start < 1500 || entries.frames.at(-1)?.physical !== physical) {
        entries.frames.push({ time, physical, folded, selected, hero: hero?.textContent,
          visibleSince, domWidth: document.documentElement.clientWidth, domHeight: document.documentElement.scrollHeight,
          committedHeight: diagnostic?.sheetCssHeight, revision: diagnostic?.readyRevision,
          meshWidth: diagnostic?.meshCssWidth, meshHeight: diagnostic?.meshCssHeight,
          textureSize: diagnostic?.textureSize, scale: diagnostic?.effectiveInkScale,
          currentRevision: diagnostic?.captureRevision });
      }
      requestAnimationFrame(sample);
    };
    requestAnimationFrame(sample);
  });
}

async function collectTrace(page, name, exercise) {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Tracing.start', {
    categories: 'toplevel,blink.user_timing,devtools.timeline,v8,disabled-by-default-devtools.timeline,disabled-by-default-devtools.screenshot,gpu',
    transferMode: 'ReturnAsStream',
  });
  const data = await exercise();
  const complete = once(cdp, 'Tracing.tracingComplete');
  await cdp.send('Tracing.end');
  const [{ stream }] = await complete;
  const gzip = createGzip();
  const file = createWriteStream(path.join(output, name + '.trace.json.gz'));
  gzip.pipe(file);
  for (;;) {
    const chunk = await cdp.send('IO.read', { handle: stream, size: 262144 });
    if (chunk.data && !gzip.write(chunk.base64Encoded ? Buffer.from(chunk.data, 'base64') : chunk.data)) {
      await once(gzip, 'drain');
    }
    if (chunk.eof) break;
  }
  gzip.end();
  await once(file, 'finish');
  await cdp.send('IO.close', { handle: stream });
  await cdp.detach();
  return data;
}

const viewport = { width: 1440, height: 900 };
const rangeMatrix = process.env.WT_TRACE_RANGES === '1';
const runs = rangeMatrix ? 1 : 3;
const rangePointer = process.env.WT_TRACE_POINTER === '1';
const startRange = process.env.WT_TRACE_START ?? '53w';
const scar = process.env.WT_TRACE_SCAR;
const scarred = ['1', 'bottom', 'both'].includes(scar);
const baseline = process.argv.includes('--baseline');
const waitReady = (page, old = -2) => page.waitForFunction(revision => {
  const paper = window.__WT_PAPER3D__?.getDiagnostics();
  return paper?.snapshotReady && paper.readyRevision > revision && paper;
}, old, { timeout: 45000 });
const elapsed = start => Math.round(performance.now() - start);
const diagnostics = page => page.evaluate(() => window.__WT_PAPER3D__.getDiagnostics());
const cold = [];
const operations = [];
let rapid;
try {
  for (let run = 0; run < (baseline ? 1 : runs); run++) {
    browser = await chromium.launch({ headless: process.env.WT_HEADFUL !== '1' });
    const context = await browser.newContext({ viewport, deviceScaleFactor: rangeMatrix ? 2 : 1, recordVideo: { dir: output, size: viewport } });
    const page = await context.newPage();
    if (scarred) await page.addInitScript(mode => {
      // Same physical line as the 1440px drag-and-reopen evidence.
      const width = 14.4, height = 16.35, edge = mode === 'both' ? 2.4 : 3.625;
      const memories = (mode === 'both' ? [false, true] : [mode === 'bottom']).map(bottom => {
        const nx = -Math.SQRT1_2, ny = bottom ? -Math.SQRT1_2 : Math.SQRT1_2;
        const origin = { x: -width / 2, y: bottom ? -height / 2 : height / 2 };
        const start = { x: origin.x + edge, y: origin.y };
        const end = { x: origin.x, y: origin.y + (bottom ? edge : -edge) };
        const length = Math.hypot(edge, edge);
        return { strength: .85, band: .2, times: 1,
          hinge: { kind: 'corner', nx, ny, depth: edge * Math.abs(ny), origin, start, end, length, span: length,
            fullSpan: true, alongShift: 0 } };
      });
      sessionStorage.setItem('wheretoken.newsprint.creases.v1', JSON.stringify({ width, height, memories }));
    }, scar);
    await setup(page);
    page.on('pageerror', error => console.error(error.message));
    if (process.env.WT_TRACE_DEBUG) page.on('console', message => console.log(message.text()));
    if (baseline) await page.addInitScript(() => {
      for (const method of ['compileShader', 'linkProgram', 'getProgramParameter', 'texImage2D',
        'texSubImage2D', 'generateMipmap', 'drawElements', 'drawArrays', 'bufferData', 'clear']) {
        const proto = WebGL2RenderingContext.prototype;
        const original = proto[method];
        if (!original) continue;
        proto[method] = function (...args) {
          const start = performance.now();
          const result = original.apply(this, args);
          if (performance.now() - start > 2) performance.measure('Baseline GL: ' + method, { start, end: performance.now() });
          return result;
        };
      }
    });

    const startup = async () => {
      const start = performance.now();
      await page.goto(rangeMatrix ? url + '&range=' + encodeURIComponent(startRange.toLowerCase()) : url, { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => document.querySelector('#ranges [aria-selected=true]'));
      const nativeMs = elapsed(start);
      try {
        await waitReady(page);
      } catch (error) {
        console.error(await page.evaluate(() => window.__WT_PAPER3D__?.getDiagnostics()));
        throw error;
      }
      return { nativeMs, readyMs: elapsed(start), diagnostic: await diagnostics(page) };
    };
    if (baseline) {
      await collectTrace(page, 'before', async () => {
        cold.push(await startup());
        await page.screenshot({ path: path.join(output, 'baseline-rest.png') });
        const start = performance.now();
        await page.getByRole('tab', { name: '7d', exact: true }).click();
        await waitReady(page, cold.at(-1).diagnostic.readyRevision);
        operations.push({ label: '7d', readyMs: elapsed(start), diagnostic: await diagnostics(page) });
      });
    } else {
      cold.push(run === 0 ? await collectTrace(page, 'after-cold', startup) : await startup());
      if (rangeMatrix) {
        const changes = rangePointer
          ? ['All', 'Today', 'All', 'Today', '30d', 'Today', '7d', '53w', '7d', '53w', '30d', '53w']
          : ['All', '7d', '30d', 'Today', 'All', 'Today', '30d', '7d', '53w', 'Today', '53w'];
        await collectTrace(page, 'after-filters', async () => {
          const rounds = Number(process.env.WT_TRACE_ROUNDS ?? 5);
          for (let round = 0; round < rounds; round++) {
            for (const label of changes) {
              let result = await page.evaluate(({ name, pointer }) => {
                const controller = window.__WT_PAPER3D__;
                const previous = controller.getDiagnostics();
                const tab = [...document.querySelectorAll('#ranges [role=tab]')].find(node => node.textContent === name);
                function measureClick(start) {
                  let firstFrame;
                  return new Promise((resolve, reject) => {
                    function inspect(now) {
                      firstFrame ??= {
                        nativeMs: now - start,
                        nativeVisible: !!document.getElementById('hero-value')?.textContent?.trim(),
                        backdropVisible: document.documentElement.classList.contains('wt-paper-flat'),
                      };
                      if (document.querySelector('#ranges [aria-selected=true]')?.textContent !== name) {
                        reject(new Error('Native range did not update by its first frame'));
                        return;
                      }
                      const diagnostic = controller.getDiagnostics();
                      if (diagnostic.readyRevision > previous.readyRevision && diagnostic.snapshotReady) {
                        const stock = document.getElementById('wt-paper-stock');
                        const viewport = stock?.getBoundingClientRect();
                        if (Math.abs(viewport?.height - diagnostic.sheetCssHeight) >= 1 || viewport?.width !== diagnostic.sheetCssWidth ||
                            diagnostic.meshCssHeight !== document.documentElement.scrollHeight ||
                            diagnostic.captureRevision !== diagnostic.readyRevision) {
                          reject(new Error('Paper atlas viewport does not match the current sheet'));
                          return;
                        }
                        resolve({ fromHeight: previous.sheetCssHeight, ...firstFrame, readyMs: performance.now() - start, diagnostic });
                      } else if (diagnostic.error || now - start > 15000) {
                        reject(new Error(JSON.stringify(diagnostic)));
                      } else {
                        requestAnimationFrame(inspect);
                      }
                    }
                    requestAnimationFrame(inspect);
                  });
                }
                if (pointer) {
                  window.paperRangeClick = new Promise((resolve, reject) => {
                    tab.addEventListener('click', event => measureClick(event.timeStamp).then(resolve, reject), { once: true });
                  });
                  return null;
                }
                const start = performance.now();
                tab.click();
                if (document.querySelector('#ranges [aria-selected=true]')?.textContent !== name) {
                  throw new Error('Native range did not update synchronously');
                }
                return measureClick(start);
              }, { name: label, pointer: rangePointer });
              if (rangePointer) {
                await page.locator('#ranges').getByRole('tab', { name: label, exact: true }).click();
                result = await page.evaluate(() => window.paperRangeClick);
              }
              operations.push({ round, label, ...result });
            }
          }
        });
      } else if (run === 2) {
        const range = name => page.locator('#ranges').getByRole('tab', { name, exact: true });
        const tab = name => page.getByRole('tab', { name, exact: true });
        await collectTrace(page, 'after-filters', async () => {
          for (let round = 0; round < 10; round++) {
            for (const [label, button] of [
              ['7d', range('7d')], ['30d', range('30d')], ['Agents', tab('Agents')],
              ['Requests', tab('Requests')], ['All', range('All')],
            ]) {
              const revision = (await diagnostics(page)).readyRevision;
              const start = performance.now();
              await button.click();
              const dispatchWallMs = elapsed(start);
              await waitReady(page, revision);
              operations.push({ round: round + 1, label, dispatchWallMs, readyMs: elapsed(start), diagnostic: await diagnostics(page) });
            }
            if (round < 9) {
              await tab('Providers').click();
              await tab('Tokens').click();
              await waitReady(page);
            }
          }
          const start = performance.now();
          const revision = (await diagnostics(page)).readyRevision;
          for (const value of ['7d', '30d', '7d']) {
            const box = await range(value).boundingBox();
            await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
          }
          const nativeMs = elapsed(start);
          await waitReady(page, revision);
          const diagnostic = await diagnostics(page);
          if (!diagnostic.documentKey.includes('range=7d')) throw new Error('Rapid clicks committed the wrong range');
          rapid = { nativeMs, readyMs: elapsed(start), diagnostic };
          await page.waitForTimeout(1500);
        });
      }
    }
    cold.at(-1).timeline = await page.evaluate(() => window.paperPerf);
    await page.screenshot({ fullPage: rangeMatrix, path: path.join(output, `rest-${run + 1}.png`) });
    const video = page.video();
    await context.close();
    await video.saveAs(path.join(output, `run-${run + 1}.webm`));
    await video.delete();
    await browser.close();
    browser = null;
  }
  await writeFile(path.join(output, 'measurements.json'), JSON.stringify({ viewport, scarred, scar, startRange, rangeMatrix, rangePointer, cold, operations, rapid }, null, 2) + '\n');
  if (rangeMatrix) {
    const warm = operations.filter(op => op.round > 0);
    if (new Set(operations.map(op => op.diagnostic.sheetCssHeight)).size !== 3 || warm.length === 0) {
      throw new Error('Matrix did not cover all three heights and warm revisits');
    }
    const misses = warm.filter(op => !op.diagnostic.stockReused || op.diagnostic.stockReadbackPixels > 0);
    console.log(`DPR2 warm stock misses: ${misses.length}/${warm.length}`);
    if (!process.env.WT_TRACE_OBSERVE) {
      if (misses.length) throw new Error('Revisited heights caused stock GPU readback');
      if (warm.some(op => !op.nativeVisible || !op.backdropVisible)) throw new Error('A warm switch hid native content or its backdrop');
      if (scar === '1' && operations.some(op => !op.diagnostic.lightReused || !op.diagnostic.creaseLighting)) {
        throw new Error('Unchanged upper crease was redrawn on a new paper height');
      }
      if (scar === 'both') {
        const newHeights = operations.filter(op => op.diagnostic.lightReadbackCount > 0);
        if (newHeights.length !== 2 || newHeights.some(op => op.diagnostic.lightReadbackCount !== 1 ||
            op.diagnostic.lightReusedCount !== 1)) throw new Error('Moving the lower crease redrew the disjoint upper crease');
      }
      if (process.env.WT_TRACE_ASSERT_LATENCY) {
        const cross = warm.filter(op => op.fromHeight !== op.diagnostic.sheetCssHeight).map(op => op.readyMs).sort((a,b) => a-b);
        const p95 = cross[Math.ceil(cross.length * .95) - 1];
        if (!(p95 <= 150)) throw new Error(`Warm cross-height p95 ${p95.toFixed(1)} ms exceeded 150 ms`);
      }
      if (startRange === 'Today') {
        const growth = operations[0].diagnostic;
        const initial = cold[0].diagnostic;
        const rows = growth.textureSize[1] - initial.textureSize[1];
        if (!growth.stockGrown || growth.stockReadbackPixels !== rows * growth.textureSize[0] ||
            growth.stockAtlasStrips !== 2) throw new Error('Stock growth did not rasterize just the new rows');
      }
    }
  }
  console.log(output, cold.map(run => [run.nativeMs, run.readyMs]), operations.length);
} finally {
  await browser?.close();
  server.close();
}
