import * as THREE from 'three';
import { PaperFoldSheet } from './scene/paper-foldsheet.js';
import { CSS_PIXELS_PER_UNIT as PIXELS_PER_UNIT, creasePositions, restoreCreases, storeCreases } from './scene/crease-memory.js';
import { buildScannedStock, toCanvas } from './paper/browser-maps.js';
import { MAX_STOCK_STRIPS, stockFrame } from './paper/paper-frame-keys.js';
import { RasterCache } from './paper/raster-cache.js';
import { creaseLightFrames, LIGHT_VERTEX_PITCH } from './paper/crease-light-frame.js';
import { tintStock } from './paper/surface-maps.js';

const canvas = new OffscreenCanvas(1, 1);
const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, .1, 250);
const scene = new THREE.Scene();
const tiles = new Map();
const ink = { value: null };
const inkAmount = { value: 0 };
const reverseInk = { value: 0 };
let renderer, sun, ground, front, back, light, saved, stockReady, stock;
const flatStock = new THREE.PlaneGeometry();
let creaseRevision = 0;
const printCache = new RasterCache(8 * 1024 * 1024, 12, () => {});
const MAX_PENDING_PRINT_BYTES = 48 * 1024 * 1024;
let pendingPrints = 0, pendingPrintBytes = 0, peakPendingPrintBytes = 0;
let current, latestRevision, model, print, scrollY, lastTime, animation, frameCount = 0;
let printUploaded = false;
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
  printUploaded = false;
}

function uploadPrint() {
  if (printUploaded || !print) return;
  renderer.initTexture(print);
  printUploaded = true;
}
function texture(source, colorSpace = THREE.NoColorSpace) {
  const result = new THREE.CanvasTexture(source);
  result.colorSpace = colorSpace;
  result.anisotropy = Math.min(4, renderer.capabilities.getMaxAnisotropy());
  return result;
}

function material(side, amount, lighting = false) {
  const stock = new THREE.MeshPhongMaterial({
    side, color: 0xffffff, shininess: 12, specular: 0x080808,
    normalScale: new THREE.Vector2(.13, .13),
  });
  stock.onBeforeCompile = shader => {
    if (lighting) {
      // Pack both optical gains in one opaque buffer for a single readback.
      // A shadow multiplies by 1-alpha; color dodge divides by 1-color.
      shader.fragmentShader = shader.fragmentShader
        .replace('#include <logdepthbuf_fragment>', `
          #include <logdepthbuf_fragment>
          if (max(abs(vNormal.x), abs(vNormal.y)) < 0.00005) {
            gl_FragColor = vec4(0.0);
            return;
          }
        `)
        .replace('#include <opaque_fragment>', `
          ReflectedLight flatReflection = ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
          #if NUM_DIR_LIGHTS > 0
            IncidentLight flatSun;
            getDirectionalLightInfo(directionalLights[0], flatSun);
            RE_Direct_BlinnPhong(flatSun, geometryPosition, normalize(mapN),
              geometryViewDir, geometryClearcoatNormal, material, flatReflection);
          #endif
          vec3 flatLight = flatReflection.directDiffuse + flatReflection.directSpecular + reflectedLight.indirectDiffuse;
          vec3 luma = vec3(0.2126, 0.7152, 0.0722);
          float base = dot(clamp(linearToOutputTexel(vec4(flatLight, 1.0)).rgb, 0.0, 1.0), luma);
          float lit = dot(clamp(linearToOutputTexel(vec4(outgoingLight, 1.0)).rgb, 0.0, 1.0), luma);
          float gain = max(0.01, lit) / max(0.01, base);
          gl_FragColor = vec4(max(0.0, 1.0 - gain), max(0.0, 1.0 - 1.0 / gain), 0.0, 1.0);
          return;
        `);
      return;
    }
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
  stock.customProgramCacheKey = () => lighting ? 'newsprint-crease-light' : 'newsprint-stock-' + side;
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
  light = new THREE.Mesh(undefined, material(THREE.FrontSide, inkAmount, true));
  light.visible = false;
  ground = new THREE.Mesh(new THREE.PlaneGeometry(), new THREE.ShadowMaterial({ opacity: .165 }));
  ground.position.z = -.045;
  ground.receiveShadow = true;
  front.castShadow = true;
  front.receiveShadow = true;
  back.receiveShadow = true;
  scene.add(ground, front, back, light);
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
    // Motion samples the analytic height field. The resting light pass
    // tessellates only the affected region, using the same solver.
    restoreCreases(saved, model, { refresh: false });
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
  camera.clearViewOffset();
  Object.assign(camera, { left: -halfWidth, right: halfWidth, top: halfHeight, bottom: -halfHeight });
  camera.position.set(0, y, Math.max(30, model.height));
  camera.far = Math.max(250, model.height * 2);
  camera.lookAt(0, y, 0);
  camera.updateProjectionMatrix();
  camera.updateMatrixWorld();
}

function configureView(layout, folding, bounds) {
  const height = folding ? layout.viewportHeight : layout.height;
  const w = bounds?.w ?? Math.floor(layout.width * layout.scale);
  const h = bounds?.h ?? Math.floor(height * layout.scale);
  if (canvas.width !== w || canvas.height !== h) renderer.setSize(w, h, false);
  view(layout, height, folding ? scrollY : 0);
  front.visible = true;
  light.visible = false;
  front.geometry = back.geometry = model.renderGeometry;
  back.visible = ground.visible = sun.castShadow = folding;
  ink.value = folding ? print : null;
  if (folding) uploadPrint();
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
    ...model.getDiagnostics(), creaseRevision, creasePositions: creasePositions(model), frameCount, geometryVertices: model.restingGeometry.attributes.position.count,
    materialMaps: true, printedInkResident: !!print, inkUploaded: printUploaded, textureSize: [print.image.width, print.image.height],
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

function creaseGeometry(layout, bounds) {
  const pixelsToCssX = layout.width / bounds.width;
  const pixelsToCssY = layout.height / bounds.height;
  const cssWidth = bounds.w * pixelsToCssX, cssHeight = bounds.h * pixelsToCssY;
  const geometry = new THREE.PlaneGeometry(cssWidth / PIXELS_PER_UNIT, cssHeight / PIXELS_PER_UNIT,
    Math.ceil(cssWidth / LIGHT_VERTEX_PITCH), Math.ceil(cssHeight / LIGHT_VERTEX_PITCH));
  const { position, normal, uv } = geometry.attributes;
  const cx = (bounds.x * pixelsToCssX + cssWidth / 2 - layout.width / 2) / PIXELS_PER_UNIT;
  const cy = (layout.height / 2 - bounds.y * pixelsToCssY - cssHeight / 2) / PIXELS_PER_UNIT;
  for (let vertex = 0; vertex < position.count; vertex++) {
    const i = vertex * 3;
    const x = position.array[i] + cx, y = position.array[i + 1] + cy;
    position.array[i] = x;
    position.array[i + 1] = y;
    uv.setXY(vertex, x / model.width + .5, y / model.height + .5);
    model.restVertex(x, y, position.array, normal.array, i);
  }
  return geometry;
}

function drawCreaseLight(layout, bounds) {
  configureView(layout, false, bounds);
  front.visible = false;
  light.visible = true;
  light.geometry = creaseGeometry(layout, bounds);
  const { x, y, w, h, width, height } = bounds;
  camera.setViewOffset(width, height, x, y, w, h);
  try {
    draw();
  } finally {
    light.geometry.dispose();
    light.geometry = undefined;
    light.visible = false;
  }
}

function readCreaseLight({ w, h }) {
  const gl = renderer.getContext();
  const gains = new Uint8Array(w * h * 4);
  gl.readPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, gains);
  const surface = new OffscreenCanvas(w, h);
  const ctx = surface.getContext('2d');
  const pixels = ctx.createImageData(w, h);
  const output = pixels.data;
  return [0, 1].map(channel => {
    for (let y = 0; y < h; y++) {
      let from = (h - 1 - y) * w * 4 + channel;
      let to = y * w * 4;
      for (let x = 0; x < w; x++, from += 4, to += 4) {
        const gain = gains[from];
        output[to] = output[to + 1] = output[to + 2] = channel ? gain : 0;
        output[to + 3] = channel ? (gain ? 255 : 0) : gain;
      }
    }
    ctx.putImageData(pixels, 0, 0);
    return surface.transferToImageBitmap();
  });
}

function sendRest(type, layout, fields, coverage, lightKeys = []) {
  const stock = stockFrame(layout);
  const previous = coverage?.key === stock.key && coverage.width === stock.width ? coverage : null;
  let bitmap;
  let drawMs = 0, stockBitmapMs = 0, lightBitmapMs = 0;
  if (!previous || previous.height < stock.height) {
    stock.start = previous?.strips < MAX_STOCK_STRIPS ? previous.height : 0;
    const rows = stock.height - stock.start;
    configureView(layout, false, { w: stock.width, h: rows });
    camera.setViewOffset(stock.width, stock.height, 0, stock.start, stock.width, rows);
    front.geometry = flatStock;
    front.scale.set(model.width, model.height, 1);
    try {
      [, drawMs] = measure('flat draw', draw);
      [bitmap, stockBitmapMs] = measure('stock bitmap', () => canvas.transferToImageBitmap());
    } finally {
      front.scale.set(1, 1, 1);
      front.geometry = model.renderGeometry;
    }
  }
  const lights = creaseLightFrames(layout, saved, creaseRevision);
  for (const lightFrame of lights) {
    if (lightKeys.includes(lightFrame.key)) continue;
    const [, lightMs] = measure('crease light draw', () => drawCreaseLight(layout, lightFrame.bounds));
    const [plates, readMs] = measure('crease light bitmap', () => readCreaseLight(lightFrame.bounds));
    drawMs += lightMs;
    lightBitmapMs += readMs;
    lightFrame.plates = plates;
  }
  post(type, { layout, bitmap, lights, diagnostics: diagnostics(),
    ...fields, stock, drawMs, stockBitmapMs, lightBitmapMs, transferMs: stockBitmapMs + lightBitmapMs },
    [bitmap, ...lights.flatMap(frame => frame.plates ?? [])].filter(Boolean));
}

function sendFrame(type, layout, fields) {
  const [bitmap, transferMs] = measure('bitmap', () => canvas.transferToImageBitmap());
  post(type, { layout, bitmap, diagnostics: diagnostics(), transferMs, ...fields }, [bitmap]);
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

async function prepare({ layout, bitmap, inkID, stock, lightKeys }) {
  const maps = await stockReady;
  if (layout.revision !== latestRevision) {
    bitmap?.close();
    post('discarded', { revision: layout.revision });
    return;
  }
  const expectedWidth = Math.floor(layout.width * layout.scale);
  const expectedHeight = Math.floor(layout.height * layout.scale);
  const inkReused = !bitmap;
  let inkDecodeMs = 0;
  if (inkReused) {
    const cached = printCache.get(inkID);
    if (!cached) {
      post('need-ink', { revision: layout.revision, inkID });
      return;
    }
    try {
      const start = performance.now();
      bitmap = await createImageBitmap(cached);
      inkDecodeMs = Math.round(performance.now() - start);
      performance.measure('Newsprint worker ink decode', { start, duration: inkDecodeMs });
      performance.clearMeasures('Newsprint worker ink decode');
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
  for (const mesh of [front, back, light]) {
    if (!mesh.material.map) mesh.material.needsUpdate = true;
    mesh.material.map = tile.albedo;
    mesh.material.normalMap = tile.normal;
    mesh.material.specularMap = tile.roughness;
  }
  for (const map of [tile.albedo, tile.normal, tile.roughness]) {
    map.repeat.set(layout.width / tile.width, layout.height / tile.height);
    map.offset.y = -map.repeat.y;
  }
  releasePrint();
  // Keep upload orientation and alpha identical to the baseline capture.
  const optical = new OffscreenCanvas(expectedWidth, expectedHeight);
  optical.getContext('bitmaprenderer').transferFromImageBitmap(bitmap);
  print = texture(optical, THREE.SRGBColorSpace);
  const first = !captureCount;
  let uploadMs = 0;
  if (first) {
    // The flat presentation uses native text. Keep the validated raster
    // resident for motion, and warm the GPU texture only for a fold.
    [, uploadMs] = measure('upload', () => {
      for (const map of [tile.albedo, tile.normal, tile.roughness]) renderer.initTexture(map);
      uploadPrint();
      renderer.getContext().finish();
    });
  }
  scrollY = layout.scrollY;
  let compileMs = 0;
  if (first) {
    const ownedPrint = print;
    configureView(layout, true);
    const start = performance.now();
    if (renderer.extensions.has('KHR_parallel_shader_compile')) await renderer.compileAsync(scene, camera);
    else renderer.compile(scene, camera);
    if (layout.revision !== latestRevision) {
      if (print === ownedPrint) releasePrint();
      post('discarded', { revision: layout.revision });
      return;
    }
    // Warm the shadowed front, reverse and depth programs before any fold.
    draw();
    compileMs = Math.round(performance.now() - start);
    performance.measure('Newsprint worker compile and warmup', { start, duration: compileMs });
  }
  captureCount++;
  current = layout;
  const readyFields = { materialMs, stockMs: maps.stockMs, geometryMs, uploadMs, compileMs, inkReused, inkDecodeMs };
  sendRest('ready', layout, readyFields, stock, lightKeys);
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
    outstanding = ++frameID;
    const fields = { folding, scrollY, motionEpoch, frameID };
    if (folding) {
      configureView(current, true);
      draw();
      sendFrame('motion', current, { ...fields, flapPoints: flapPoints(current) });
    } else {
      sendRest('motion', current, fields, stockFrame(current));
    }
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
      releaseFold();
    }
    if (current) releasePrint();
    current = null;
    stopAnimation();
    return;
  }
  if (message.type === 'prepare') return prepare(message);
  if (message.type === 'visibility') {
    visible = message.visible;
    if (!visible) stopAnimation();
    else if (model?.phase === 'settling') wake();
    return;
  }
  if (!current || message.revision !== current.revision) return;
  motionEpoch = message.motionEpoch;
  if (message.type === 'warm-ink') {
    measure('fold ink upload', uploadPrint);
  } else if (message.type === 'scroll') {
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
    releaseFold();
    wake();
  } else if (message.type === 'reset') {
    model.reset();
    wake();
  }
}

function releaseFold() {
  const before = model.memories.slice();
  model.release();
  const updated = storeCreases(model, before, saved);
  if (updated === saved) return;
  saved = updated;
  creaseRevision++;
  // A previous renderer can still read this tab's visible marks. Only
  // the anchored records are authoritative for this renderer.
  const legacy = { width: model.width, height: model.height,
    memories: model.memories.map(({ hinge, strength, band, times }) => ({ hinge, strength, band, times })) };
  post('creases', { saved: { ...legacy, anchored: saved }, creaseRevision });
}

self.onmessage = ({ data }) => receive(data).catch(reportError);
