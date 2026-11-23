// Exercise placeholder compositing independently of the product renderer.
import { chromium, firefox, webkit } from '@playwright/test';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const output = path.resolve(process.argv[2]);
const workerSource = `
let surfaces;
self.onmessage = ({data}) => {
  if (data.surfaces) {
    surfaces = data.surfaces.map(canvas => ({canvas, gl: canvas.getContext('webgl2')}));
    postMessage({available: surfaces.every(surface => surface.gl)});
    return;
  }
  const {canvas, gl} = surfaces[data.slot];
  const start = performance.now();
  gl.viewport(0, 0, canvas.width, canvas.height);
  gl.clearColor(...data.color, 1);
  gl.clear(gl.COLOR_BUFFER_BIT);
  gl.finish();
  const drawMs = performance.now() - start;
  const bitmap = data.direct ? null : canvas.transferToImageBitmap();
  postMessage({bitmap, drawMs, workerMs: performance.now() - start}, bitmap ? [bitmap] : []);
};
`;
const html = '<!doctype html><meta charset=utf-8><style>body{margin:0;background:white}canvas{position:absolute;inset:0;visibility:hidden}</style>';
const server = createServer((req, res) => {
  if (req.url === '/worker.js') res.writeHead(200, { 'Content-Type': 'text/javascript' }).end(workerSource);
  else res.writeHead(200, { 'Content-Type': 'text/html' }).end(html);
});
await mkdir(output, { recursive: true });
server.listen(0, '127.0.0.1');
await once(server, 'listening');

async function initialize(page, direct) {
  return page.evaluate(async direct => {
    const width = 1440, height = 1635;
    const canvases = Array.from({ length: 2 }, () => {
      const canvas = document.createElement('canvas');
      canvas.width = width;
      canvas.height = height;
      document.body.append(canvas);
      return canvas;
    });
    const surfaces = direct
      ? canvases.map(canvas => canvas.transferControlToOffscreen())
      : [new OffscreenCanvas(width, height), new OffscreenCanvas(width, height)];
    const worker = new Worker('/worker.js');
    const request = (message, transfer = []) => new Promise((resolve, reject) => {
      worker.onmessage = event => resolve(event.data);
      worker.onerror = event => reject(new Error(event.message || 'Worker failed'));
      worker.postMessage(message, transfer);
    });
    window.probe = async (slot, color) => {
      const start = performance.now();
      const result = await request({ slot, color, direct });
      const replyMs = performance.now() - start;
      await new Promise(requestAnimationFrame);
      if (result.bitmap) canvases[slot].getContext('bitmaprenderer').transferFromImageBitmap(result.bitmap);
      canvases.forEach((canvas, index) => { canvas.style.visibility = index === slot ? 'visible' : 'hidden'; });
      return { drawMs: result.drawMs, workerMs: result.workerMs, replyMs, showMs: performance.now() - start };
    };
    return (await request({ surfaces }, surfaces)).available;
  }, direct);
}

const results = [];
try {
  for (const engine of [chromium, firefox, webkit]) {
    const browser = await engine.launch({ headless: true });
    try {
      for (const direct of [false, true]) {
        const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
        await page.goto('http://127.0.0.1:' + server.address().port);
        const row = { engine: engine.name(), mode: direct ? 'dual placeholder' : 'bitmap', available: false, samples: [] };
        results.push(row);
        try {
          row.available = await initialize(page, direct);
          if (!row.available) continue;
          for (let frame = 0; frame < 12; frame++) {
            const color = [0, 1, 2].map(channel => Number(channel === frame % 3));
            const result = await page.evaluate(([slot, rgb]) => window.probe(slot, rgb), [frame % 2, color]);
            const screenshot = await page.screenshot({ clip: { x: 160, y: 160, width: 8, height: 8 } });
            const pixel = await page.evaluate(async encoded => {
              const png = Uint8Array.from(atob(encoded), value => value.charCodeAt(0));
              const image = await createImageBitmap(new Blob([png], { type: 'image/png' }));
              const ctx = new OffscreenCanvas(1, 1).getContext('2d');
              ctx.drawImage(image, 0, 0);
              image.close();
              return [...ctx.getImageData(0, 0, 1, 1).data];
            }, screenshot.toString('base64'));
            row.samples.push({ ...result, pixel, expectedFrame: pixel.slice(0, 3).every((value, i) => value === color[i] * 255) });
            if (frame < 3) await writeFile(path.join(output, `${engine.name()}-${direct ? 'direct' : 'bitmap'}-${frame}.png`),
              await page.screenshot());
          }
        } catch (error) {
          row.error = error.message;
        } finally {
          await page.close();
        }
      }
    } finally {
      await browser.close();
    }
  }
  await writeFile(path.join(output, 'measurements.json'), JSON.stringify(results, null, 2) + '\n');
} finally {
  server.close();
}
