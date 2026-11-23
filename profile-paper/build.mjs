import { build } from 'esbuild';
import { writeFile, readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { resolve, dirname } from 'node:path';
import { incrementalCapture } from './capture-build.mjs';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const assets = resolve(root, 'internal/profilewebembed/static/assets');
const hash = body => createHash('sha256').update(body).digest('hex');

async function bundle(entry, file, options = {}) {
  const result = await build({
    entryPoints: [resolve(root, 'profile-paper/src', entry)], bundle: true,
    format: 'iife', platform: 'browser', target: 'es2022', minify: true,
    legalComments: 'inline', write: false, ...options,
  });
  const bytes = result.outputFiles[0].contents;
  await writeFile(resolve(assets, file), bytes);
  console.log(file, bytes.length, 'bytes');
  return hash(bytes);
}

const worker = await bundle('paper-worker.js', 'paper-worker.js');
const controller = await bundle('live-paper.js', 'paper-3d.js', {
  plugins: [incrementalCapture],
  alias: { html2canvas: resolve(root, 'profile-paper/node_modules/html2canvas/dist/html2canvas.esm.js') },
  define: { PAPER_WORKER_FILE: JSON.stringify('paper-worker.js?v=' + worker) },
});
const loaderPath = resolve(assets, 'paper.js');
const loader = await readFile(loaderPath, 'utf8');
const updated = loader.replace(/paper-3d[.]js[?]v=(?:__PAPER3D_SHA256__|[0-9a-f]{64})/, 'paper-3d.js?v=' + controller);
if (!updated.includes('paper-3d.js?v=' + controller)) throw new Error('Missing hashed 3D script loader');
await writeFile(loaderPath, updated);

const indexPath = resolve(assets, '../index.html');
let index = await readFile(indexPath, 'utf8');
for (const name of ['paper.js', 'profile.css', 'profile.js']) {
  const prefix = './assets/' + name + '?v=';
  const body = (await readFile(resolve(assets, name), 'utf8')).replaceAll('\r\n', '\n');
  index = index.replace(/\.\/assets\/(?:paper\.js|profile\.css|profile\.js)\?v=[0-9a-f]{64}/g,
    match => match.startsWith(prefix) ? prefix + hash(body) : match);
  if (!index.includes(prefix + hash(body))) throw new Error('Missing hashed ' + name);
}
await writeFile(indexPath, index);
