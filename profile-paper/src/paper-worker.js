import * as THREE from 'three';
import { PaperFoldSheet } from './scene/paper-foldsheet.js';
import { restoreCreases } from './scene/crease-memory.js';
import { buildScannedStock, toCanvas } from './paper/browser-maps.js';
import { RasterCache } from './paper/raster-cache.js';
import { tintStock } from './paper/surface-maps.js';

const PIXELS_PER_UNIT = 100;
const MAX_PIXELS = 12_000_000;
const canvas = new OffscreenCanvas(1, 1);
const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, .1, 250);
const scene = new THREE.Scene();
const tiles = new Map();
const ink = { value: null };
const inkAmount = { value: 0 };
const reverseInk = { value: 0 };
let renderer, sun, ground, front, back, saved, stockReady, stock;
let creaseRevision = 0;
const printCache = new RasterCache(8 * 1024 * 1024, 12, () => {});
const MAX_PENDING_PRINT_BYTES = 48 * 1024 * 1024;
let pendingPrints = 0, pendingPrintBytes = 0, peakPendingPrintBytes = 0;
let current, readyFields, latestRevision, model, print, scrollY, lastTime, animation, frameCount = 0;
let captureCount = 0, materialBuilds = 0, fastInkUpdates = 0;
let visible = true;
let motionEpoch = 0, motionDirty = false, frameID = 0, outstanding = 0;

function post(type, fields = {}, transfer = []) {
  self.postMessage({ type, ...fields }, transfer);
}
function reportError(error) {
  post('failed', { error: error?.message || String(error) });
}
function disposeMesh(sheet) {
  sheet?.restingGeometry.dispose();
  sheet?.foldedGeometry.dispose();
}
function releasePrint() {
  print?.dispose();
  print = ink.value = null;
}
function texture(source, colorSpace = THREE.NoColorSpace) {
  const result = new THREE.CanvasTexture(source);
  result.colorSpace = colorSpace;
  result.anisotropy = Math.min(4, renderer.capabilities.getMaxAnisotropy());
  return result;
}

function material(side, amount) {
  const stock = new THREE.MeshPhongMaterial({
    side, color: 0xffffff, shininess: 12, specular: 0x080808,
    normalScale: new THREE.Vector2(.13, .13),
  });
  stock.onBeforeCompile = shader => {
    Object.assign(shader.uniforms, { paperInk: ink, paperInkAmount: amount });
    shader.vertexShader = 'varying vec2 printUV;\n' + shader.vertexShader
      .replace('#include <uv_vertex>', '#include <uv_vertex>\nprintUV = uv;');
    if (side === THREE.BackSide) {
      shader.vertexShader = shader.vertexShader.replace('#include <begin_vertex>',
        '#include <begin_vertex>\ntransformed -= normal * 0.007;');
    }
    shader.fragmentShader = 'varying vec2 printUV;\nuniform sampler2D paperInk;\nuniform float paperInkAmount;\n' +
      shader.fragmentShader.replace('#include <map_fragment>', `
        #include <map_fragment>
        if (paperInkAmount > 0.0) {
          vec4 printed = texture2D(paperInk, printUV);
          diffuseColor.rgb = mix(diffuseColor.rgb, printed.rgb, printed.a * paperInkAmount);
        }
      `);
  };
  stock.customProgramCacheKey = () => 'newsprint-stock-' + side;
  return stock;
}

async function initialize(message) {
  saved = message.creases;
  const gl = canvas.getContext('webgl2', { alpha: true, antialias: true, depth: true });
  if (!gl) throw new Error('Worker WebGL2 unavailable');
  canvas.addEventListener('webglcontextlost', event => {
    event.preventDefault();
    reportError(new Error('Worker WebGL context lost'));
  });
  renderer = new THREE.WebGLRenderer({ canvas, context: gl });
  renderer.debug.onShaderError = () => { throw new Error('Newsprint shader could not link'); };
  renderer.setClearColor(0xffffff, 0);
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = THREE.PCFShadowMap;
  renderer.shadowMap.autoUpdate = false;
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  scene.add(new THREE.AmbientLight('#ffffff', 2.27));
  sun = new THREE.DirectionalLight('#fff8ee', 1.25);
  sun.position.set(-12, 8, 7);
  sun.shadow.mapSize.set(1024, 1024);
  sun.shadow.camera.near = .1;
  sun.shadow.camera.far = 100;
  sun.shadow.bias = -.00012;
  sun.shadow.normalBias = .019;
  scene.add(sun);
  front = new THREE.Mesh(undefined, material(THREE.FrontSide, inkAmount));
  back = new THREE.Mesh(undefined, material(THREE.BackSide, reverseInk));
  ground = new THREE.Mesh(new THREE.PlaneGeometry(), new THREE.ShadowMaterial({ opacity: .165 }));
  ground.position.z = -.045;
  ground.receiveShadow = true;
  front.castShadow = true;
  front.receiveShadow = true;
  back.receiveShadow = true;
  scene.add(ground, front, back);
  stockReady = (async () => {
    const response = await fetch('./newsprint-height.jpg');
    if (!response.ok) throw new Error('Newsprint height scan unavailable');
    const scan = await createImageBitmap(await response.blob());
    try {
      const [prepared, stockMs] = measure('stock', () => buildScannedStock(scan));
      stock = prepared;
      return {
        normal: repeatMap(toCanvas(stock.normal)),
        roughness: repeatMap(toCanvas(stock.roughness)),
        stockMs,
      };
    } finally {
      scan.close();
    }
  })();
  post('available', {
    renderer: gl.getParameter(gl.getExtension('WEBGL_debug_renderer_info')?.UNMASKED_RENDERER_WEBGL || gl.RENDERER),
    limit: Math.min(renderer.capabilities.maxTextureSize, gl.getParameter(gl.MAX_RENDERBUFFER_SIZE)),
  });
  await stockReady;
}

function repeatMap(source, colorSpace) {
  const map = texture(source, colorSpace);
  map.wrapS = map.wrapT = THREE.RepeatWrapping;
  return map;
}

function getTile(layout, maps) {
  const key = layout.rgb.join(',');
  if (tiles.has(key)) return tiles.get(key);
  const tile = {
    width: stock.width, height: stock.height,
    albedo: repeatMap(toCanvas(tintStock(stock, layout.rgb)), THREE.SRGBColorSpace),
    normal: maps.normal, roughness: maps.roughness,
  };
  tiles.set(key, tile);
  if (tiles.size > 2) {
    const oldest = tiles.keys().next().value;
    const maps = tiles.get(oldest);
    maps.albedo.dispose();
    tiles.delete(oldest);
  }
  materialBuilds++;
  return tile;
}

function updateGeometry(layout) {
  const width = layout.width / PIXELS_PER_UNIT, height = layout.height / PIXELS_PER_UNIT;
  if (!model || model.width !== width || model.height !== height) {
    const previous = model;
    model = new PaperFoldSheet({ width, height });
    restoreCreases(saved, model);
    disposeMesh(previous);
    ground.geometry.dispose();
    ground.geometry = new THREE.PlaneGeometry(width, height);
    const bound = Math.max(width, height) * 1.25;
    Object.assign(sun.shadow.camera, { left: -bound, right: bound, top: bound, bottom: -bound });
    sun.shadow.camera.updateProjectionMatrix();
  } else if (model.phase !== 'resting') {
    model.reset();
  }
  front.geometry = back.geometry = model.renderGeometry;
}

function view(layout, height, offset) {
  const halfWidth = layout.width / PIXELS_PER_UNIT / 2;
  const halfHeight = height / PIXELS_PER_UNIT / 2;
  const y = model.height / 2 - offset / PIXELS_PER_UNIT - halfHeight;
  Object.assign(camera, { left: -halfWidth, right: halfWidth, top: halfHeight, bottom: -halfHeight });
  camera.position.set(0, y, Math.max(30, model.height));
  camera.far = Math.max(250, model.height * 2);
  camera.lookAt(0, y, 0);
  camera.updateProjectionMatrix();
  camera.updateMatrixWorld();
}

function configureView(layout, folding) {
  const height = folding ? layout.viewportHeight : layout.height;
  const w = Math.floor(layout.width * layout.scale);
  const h = Math.floor(height * layout.scale);
  if (canvas.width !== w || canvas.height !== h) renderer.setSize(w, h, false);
  view(layout, height, folding ? scrollY : 0);
  front.geometry = back.geometry = model.renderGeometry;
  back.visible = ground.visible = sun.castShadow = folding;
  inkAmount.value = folding ? 1 : 0;
  reverseInk.value = folding ? .11 : 0;
  renderer.shadowMap.needsUpdate = folding;
}

function measure(name, fn) {
  const start = performance.now();
  const value = fn();
  const end = performance.now();
  performance.clearMeasures('Newsprint worker ' + name);
  performance.measure('Newsprint worker ' + name, { start, end });
  return [value, Math.round(end - start)];
}

function draw() {
  renderer.render(scene, camera);
  // Synchronize on the worker before handing off the GPU resource. Otherwise
  // the compositor can inherit pending work at the point of bitmap display.
  const gl = renderer.getContext();
  gl.finish();
  if (gl.isContextLost()) throw new Error('Worker WebGL context lost');
  frameCount++;
}

function diagnostics() {
  return {
    ...model.getDiagnostics(), frameCount, geometryVertices: model.restingGeometry.attributes.position.count,
    materialMaps: true, printedInkResident: !!print, textureSize: [print.image.width, print.image.height],
    meshCssWidth: Math.round(model.width * PIXELS_PER_UNIT),
    meshCssHeight: Math.round(model.height * PIXELS_PER_UNIT),
    fastInkUpdates, fullMaterialBuilds: materialBuilds, inkCacheBytes: printCache.bytes, pendingPrintBytes, peakPendingPrintBytes,
  };
}

function flapPoints(layout) {
  if (!model.hinge) return [];
  const points = [];
  for (const [u, v] of [[0, 0], [0, 1], [1, 0], [1, 1]]) {
    const x = (u - .5) * model.width, y = (v - .5) * model.height;
    if (model.q({ x, y }) < model.bandWidth * .3) continue;
    const mapped = model.mapVertex(x, y);
    const position = new THREE.Vector3(mapped.x, mapped.y, mapped.z).project(camera);
    points.push({ u, v, x: (position.x + 1) * layout.width / 2, y: (1 - position.y) * layout.viewportHeight / 2 });
  }
  return points;
}

function stockKey(layout) {
  return JSON.stringify([layout.width, layout.height, layout.scale, layout.rgb, creaseRevision]);
}

function cropSource(layout, stockKeys) {
  if (model.memories.length || !Number.isInteger(layout.scale)) return undefined;
  // A flat stock sample is periodic, with vertical phase fixed at the
  // bottom edge by the sheet UV. The bottom rows of a taller homogeneous
  // sheet therefore cover the shorter sheet at the same texel pitch.
  let sourceHeight = Infinity;
  for (const key of stockKeys) {
    const [, height] = JSON.parse(key);
    if (height > layout.height && height < sourceHeight && key === stockKey({ ...layout, height })) {
      sourceHeight = height;
    }
  }
  return Number.isFinite(sourceHeight) ? stockKey({ ...layout, height: sourceHeight }) : undefined;
}

function sendReady(layout, fields, stockKeys = []) {
  const key = stockKey(layout);
  const cropFrom = stockKeys.includes(key) ? undefined : cropSource(layout, stockKeys);
  if (cropFrom || stockKeys.includes(key)) {
    post('ready', { layout, diagnostics: diagnostics(), ...fields, stockKey: key, cropFrom, drawMs: 0, transferMs: 0 });
    return;
  }
  configureView(layout, false);
  const [, drawMs] = measure('flat draw', draw);
  sendFrame('ready', layout, { ...fields, drawMs });
}

function sendFrame(type, layout, fields) {
  const [bitmap, transferMs] = measure('bitmap', () => canvas.transferToImageBitmap());
  post(type, { layout, bitmap, diagnostics: diagnostics(), transferMs, stockKey: stockKey(layout), ...fields }, [bitmap]);
}

function rememberPrint(id, canvas) {
  const bytes = canvas.width * canvas.height * 4;
  if (!canvas.convertToBlob || pendingPrints >= 2 || pendingPrintBytes + bytes > MAX_PENDING_PRINT_BYTES) return;
  pendingPrints++;
  pendingPrintBytes += bytes;
  peakPendingPrintBytes = Math.max(peakPendingPrintBytes, pendingPrintBytes);
  // Encoding is optional and never delays a new revision waiting on ink.
  Promise.resolve().then(() => canvas.convertToBlob({ type: 'image/png' })).then(blob => {
    if (printCache.put(id, blob, blob.size)) post('ink-cache', { ids: Array.from(printCache.keys()) });
  }).catch(() => {}).finally(() => {
    pendingPrints--;
    pendingPrintBytes -= bytes;
  });
}

async function prepare({ layout, bitmap, inkID, stockKeys }) {
  const maps = await stockReady;
  if (layout.revision !== latestRevision) {
    bitmap?.close();
    post('discarded', { revision: layout.revision });
    return;
  }
  const expectedWidth = Math.floor(layout.width * layout.scale);
  const expectedHeight = Math.floor(layout.height * layout.scale);
  const inkReused = !bitmap;
  if (inkReused) {
    const cached = printCache.get(inkID);
    if (!cached) {
      post('need-ink', { revision: layout.revision, inkID });
      return;
    }
    try {
      bitmap = await createImageBitmap(cached);
    } catch {
      printCache.delete(inkID);
      post('need-ink', { revision: layout.revision, inkID });
      return;
    }
    if (layout.revision !== latestRevision) {
      bitmap.close();
      post('discarded', { revision: layout.revision });
      return;
    }
  }
  const source = bitmap;
  if (source.width !== expectedWidth || source.height !== expectedHeight) {
    bitmap?.close();
    throw new Error('Paper capture size mismatch');
  }
  stopAnimation();
  current = null;
  const previousMaterialBuilds = materialBuilds;
  const [tile, materialMs] = measure('material', () => getTile(layout, maps));
  if (previousMaterialBuilds === materialBuilds) fastInkUpdates++;
  const [, geometryMs] = measure('geometry', () => updateGeometry(layout));
  for (const mesh of [front, back]) {
    if (!mesh.material.map) mesh.material.needsUpdate = true;
    mesh.material.map = tile.albedo;
    mesh.material.normalMap = tile.normal;
    mesh.material.specularMap = tile.roughness;
  }
  for (const map of [tile.albedo, tile.normal, tile.roughness]) {
    map.repeat.set(layout.width / tile.width, layout.height / tile.height);
  }
  releasePrint();
  // Keep upload orientation and alpha identical to the baseline capture.
  const optical = new OffscreenCanvas(expectedWidth, expectedHeight);
  optical.getContext('bitmaprenderer').transferFromImageBitmap(bitmap);
  print = texture(optical, THREE.SRGBColorSpace);
  ink.value = print;
  const [, uploadMs] = measure('upload', () => {
    for (const map of [tile.albedo, tile.normal, tile.roughness, print]) renderer.initTexture(map);
    renderer.getContext().finish();
  });
  const first = !captureCount;
  scrollY = layout.scrollY;
  let compileMs = 0;
  if (first) {
    configureView(layout, true);
    const start = performance.now();
    if (renderer.extensions.has('KHR_parallel_shader_compile')) await renderer.compileAsync(scene, camera);
    else renderer.compile(scene, camera);
    // Warm the shadowed front, reverse and depth programs before any fold.
    draw();
    compileMs = Math.round(performance.now() - start);
    performance.measure('Newsprint worker compile and warmup', { start, duration: compileMs });
  }
  captureCount++;
  if (layout.revision !== latestRevision) {
    releasePrint();
    post('discarded', { revision: layout.revision });
    return;
  }
  current = layout;
  readyFields = { materialMs, stockMs: maps.stockMs, geometryMs, uploadMs, compileMs, inkReused };
  sendReady(layout, readyFields, stockKeys);
  if (!inkReused && inkID !== null) rememberPrint(inkID, optical);
}

function stopAnimation() {
  if (animation) cancelAnimationFrame(animation);
  animation = 0;
  lastTime = 0;
  motionDirty = false;
  outstanding = 0;
}

function scheduleMotion() {
  if (motionDirty && !outstanding && !animation && current && visible) animation = requestAnimationFrame(tick);
}

function wake() {
  motionDirty = true;
  scheduleMotion();
}

function tick(now) {
  animation = 0;
  motionDirty = false;
  if (!current || !visible) return;
  try {
    const dt = lastTime ? THREE.MathUtils.clamp((now - lastTime) / 1000, 1/240, .10) : 1/60;
    lastTime = now;
    model.step(dt);
    const folding = model.phase !== 'resting';
    configureView(current, folding);
    draw();
    outstanding = ++frameID;
    sendFrame('motion', current, { folding, scrollY, motionEpoch, frameID,
      flapPoints: folding ? flapPoints(current) : [] });
    const bending = model.phase === 'dragging' &&
      (Math.abs(model.target - model.theta) > .003 || Math.abs(model.velocity) > .008);
    if (bending || model.phase === 'settling') wake();
    else lastTime = 0;
  } catch (error) {
    reportError(error);
  }
}

function regrab(contact) {
  if (model.phase !== 'latched') return contact;
  view(current, current.viewportHeight, scrollY);
  const ray = new THREE.Raycaster();
  ray.setFromCamera(new THREE.Vector2(contact.x / current.width * 2 - 1,
    1 - contact.y / current.viewportHeight * 2), camera);
  const hit = ray.intersectObjects([front, back], false).find(point => point.uv &&
    model.q({ x: (point.uv.x - .5) * model.width, y: (point.uv.y - .5) * model.height }) > model.bandWidth * .30);
  return hit ? { u: hit.uv.x, v: hit.uv.y } : flapPoints(current)
    .find(point => Math.hypot(point.x - contact.x, point.y - contact.y) < 28);
}

async function receive(message) {
  if (message.type === 'init') return initialize(message);
  if (message.type === 'frame-consumed') {
    if (outstanding === message.frameID) {
      outstanding = 0;
      scheduleMotion();
    }
    return;
  }
  if (message.type === 'invalidate') {
    latestRevision = message.revision;
    if (model?.phase === 'dragging') {
      model.release();
      storeCreases();
    }
    if (current) releasePrint();
    current = null;
    stopAnimation();
    return;
  }
  if (message.type === 'prepare') return prepare(message);
  if (message.type === 'render-stock') {
    if (current?.revision === message.revision) sendReady(current, readyFields);
    return;
  }
  if (message.type === 'visibility') {
    visible = message.visible;
    if (!visible) stopAnimation();
    else if (model?.phase === 'settling') wake();
    return;
  }
  if (!current || message.revision !== current.revision) return;
  motionEpoch = message.motionEpoch;
  if (message.type === 'scroll') {
    scrollY = message.scrollY;
    if (model.phase !== 'resting') wake();
  } else if (message.type === 'grab') {
    scrollY = message.scrollY;
    const contact = regrab(message.contact);
    if (contact && model.grabUV(contact.u, contact.v, message.time)) wake();
  } else if (message.type === 'drag') {
    model.dragDelta(message.dx, message.dy, message.time);
    wake();
  } else if (message.type === 'release') {
    model.release();
    storeCreases();
    wake();
  } else if (message.type === 'reset') {
    model.reset();
    wake();
  }
}

function storeCreases() {
  if (!model.memories.length) return;
  saved = { width: model.width, height: model.height, memories: model.memories };
  creaseRevision++;
  post('creases', { saved });
}

self.onmessage = ({ data }) => receive(data).catch(reportError);
