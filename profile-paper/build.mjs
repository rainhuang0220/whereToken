import { build } from 'esbuild';
import { mkdir, writeFile, readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { resolve, dirname } from 'node:path';
const root=resolve(dirname(fileURLToPath(import.meta.url)), '..');
const output=resolve(root,'internal/profilewebembed/static/assets/paper-3d.js');
const buildResult=await build({entryPoints:[resolve(root,'profile-paper/src/live-paper.js')],bundle:true,
  format:'iife',platform:'browser',target:'es2022',minify:true,legalComments:'inline',write:false});
const bytes=buildResult.outputFiles[0].contents;
await mkdir(dirname(output),{recursive:true});
await writeFile(output,bytes);
const hash=createHash('sha256').update(bytes).digest('hex');
const loaderPath=resolve(root,'internal/profilewebembed/static/assets/paper.js');
const loader=await readFile(loaderPath,'utf8');
const updated=loader.replace(/paper-3d[.]js[?]v=(?:__PAPER3D_SHA256__|[0-9a-f]{64})/,
  'paper-3d.js?v='+hash);
if(updated===loader && !loader.includes('paper-3d.js?v='+hash))
  throw new Error('Missing hashed 3D script loader');
if(updated!==loader)await writeFile(loaderPath,updated);

console.log('paper-3d.js',bytes.length,'bytes');
