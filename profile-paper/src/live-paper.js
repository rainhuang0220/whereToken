import { creaseLightFrames } from './paper/crease-light-frame.js';
import { stockFrame } from './paper/paper-frame-keys.js';
import { RasterCache } from './paper/raster-cache.js';
import { captureStyleKey } from './paper/capture-style.js';
import { captureProfile } from './paper/capture-profile.js';
import { CREASE_SESSION_KEY, readCreases } from './scene/crease-memory.js';

const root = document.documentElement;
const fine = matchMedia('(pointer:fine)');
const reduced = matchMedia('(prefers-reduced-motion:reduce)');
const small = matchMedia('(max-width:700px)');
const darkScheme = matchMedia('(prefers-color-scheme:dark)');
const controls = 'button,a,input,textarea,select,summary,[role=tab],[role=button],[contenteditable],dialog';
const MAX_PIXELS = 12_000_000;
const workerURL = new URL(PAPER_WORKER_FILE, document.currentScript.src);

let worker, limit, renderer, observer, degraded, capture, captureAbort, pendingFrame, scheduled = 0;
let revision = 0, layout, committed, drag, rest, fold, shownScroll, foldShown = false;
let lighting = new Map();
let lightLayer, savedCreases, creaseRevision = 0;
let skippedFrames = 0, lastError, captureTimes, workerTimes, dprQuery;
let latestMotion, motionEpoch = 0, motionActive = false;
const inkIndex = new Map();
const regionCache = new Map();
let nextInkID = 0;
let cachedInkIDs = new Set();
const stockFrames = new RasterCache(48 * 1024 * 1024, 6, disposeStock);
const lightFrames = new RasterCache(16 * 1024 * 1024, 6, frame => {
  // A cache eviction must not invalidate a currently presented patch.
  if (lighting.get(frame.key) !== frame) disposeLighting(frame);
});

function disposeLayer(frame) {
  frame.node.remove();
  frame.canvas.width = frame.canvas.height = 0;
}

function disposeStock(atlas) {
  atlas.node.remove();
  for (const strip of atlas.strips) disposeLayer(strip);
}

function detachStock() {
  if (!rest) return;
  if (stockFrames.owns(rest.key, rest)) rest.node.remove();
  else disposeStock(rest);
  rest = null;
}

function disposeLighting(frame) {
  frame.node.remove();
  for (const plane of frame.planes) disposeLayer(plane);
}

function availableLight(key) {
  return lighting.get(key) ?? lightFrames.get(key);
}

function detachLight() {
  presentLight([], null);
}

function closeBitmaps(message) {
  message.bitmap?.close();
  for (const frame of message.lights ?? []) {
    for (const bitmap of frame.plates ?? []) bitmap.close();
  }
}

// Capture keys use the rendered DOM rather than a filter name. Runtime
// labels, language, hosted refresh, dialogs and open details can all change
// the ink without changing the URL.
function inkFingerprint(target, styleKey) {
  // These elements can change their pixels without a markup change.
  if (!styleKey || document.querySelector('main img, main canvas, main video, main iframe, main input, main textarea, main select')) return null;
  const content = Array.from(document.body.children)
    .filter(node => !['SCRIPT', 'IFRAME'].includes(node.tagName) &&
      !['wt-paper-stock', 'wt-paper-light', 'wt-paper-fold'].includes(node.id))
    .map(node => node.outerHTML);
  const heat = document.getElementById('heat-scroll');
  return JSON.stringify([
    styleKey, target.rgb, target.width, target.height, target.viewportWidth,
    target.viewportHeight, target.dpr, target.scale,
    heat?.scrollLeft, heat?.scrollTop, content,
  ]);
}

// CSS paints the native document throughout startup and invalidation. The
// only path onto the screen is a fully rendered bitmap for the measured DOM.
function enabled() {
  return root.dataset.activityPalette === 'newsprint' && fine.matches && !reduced.matches && !small.matches;
}

function measure() {
  const width = Math.max(1, root.clientWidth);
  const viewportWidth = Math.max(1, innerWidth);
  const height = Math.max(innerHeight, root.scrollHeight, document.body.scrollHeight);
  const viewportHeight = Math.max(1, innerHeight);
  const dpr = devicePixelRatio || 1;
  const scale = Math.min(dpr, 2, Math.sqrt(MAX_PIXELS / (width * height)), limit / width, limit / height);
  const color = getComputedStyle(document.body).backgroundColor;
  const rgb = color.match(/[\d.]+/g).slice(0, 3).map(Number);
  const documentKey = [location.search, root.lang, root.dataset.activityPalette,
    root.dataset.theme, color, root.dataset.profileSnapshot].join('|');
  return { revision, width, height, viewportWidth, viewportHeight, dpr, scale, documentKey, rgb, scrollY };
}

function sameLayout(a, b) {
  return a.revision === b.revision && a.width === b.width && a.height === b.height &&
    a.viewportWidth === b.viewportWidth && a.viewportHeight === b.viewportHeight && a.dpr === b.dpr && a.documentKey === b.documentKey;
}

function layer(id) {
  const canvas = document.createElement('canvas');
  if (id) canvas.id = id;
  canvas.setAttribute('aria-hidden', 'true');
  const context = canvas.getContext('bitmaprenderer');
  if (!context) throw new Error('ImageBitmap presentation unavailable');
  return { canvas, context, node: canvas };
}

function paint(target, bitmap, width, height) {
  const { width: pixelsWide, height: pixelsHigh } = bitmap;
  if (target.canvas.width !== pixelsWide) target.canvas.width = pixelsWide;
  if (target.canvas.height !== pixelsHigh) target.canvas.height = pixelsHigh;
  if (target.width !== width) {
    target.width = width;
    target.canvas.style.width = width + 'px';
  }
  if (target.height !== height) {
    target.height = height;
    target.canvas.style.height = height + 'px';
  }
  target.context.transferFromImageBitmap(bitmap);
  if (target.canvas.width !== pixelsWide || target.canvas.height !== pixelsHigh) {
    throw new Error('Paper bitmap backing store size mismatch');
  }
}

function showFold(visible) {
  if (foldShown === visible) return;
  foldShown = visible;
  root.classList.toggle('wt-paper-folding', visible);
}

function hide(keepBackdrop = false) {
  showFold(false);
  root.classList.remove('wt-physical-print', 'wt-paper-flat');
  if (!keepBackdrop) {
    detachStock();
    detachLight();
  }
  if (fold) disposeLayer(fold);
  fold = null;
  committed = latestMotion = null;
  motionActive = false;
  if (pendingFrame) discardFrame(pendingFrame);
  pendingFrame = null;
}

function invalidate() {
  if (!worker) return;
  revision++;
  captureAbort?.abort();
  drag = null;
  layout = null;
  hide(true);
  worker.postMessage({ type: 'invalidate', revision });
  schedule();
}

function schedule() {
  if (!scheduled && worker && !document.hidden) scheduled = requestAnimationFrame(advance);
}

function fail(error) {
  lastError = typeof error === 'string' ? error : error.message;
  degraded = true;
  stop();
}

function stop() {
  worker?.terminate();
  worker = null;
  captureAbort?.abort();
  captureAbort = null;
  observer?.disconnect();
  observer = layout = capture = null;
  renderer = limit = captureTimes = workerTimes = null;
  drag = null;
  if (scheduled) cancelAnimationFrame(scheduled);
  scheduled = 0;
  hide();
  creaseRevision = 0;
  savedCreases = null;
  stockFrames.clear();
  lightFrames.clear();
  inkIndex.clear();
  cachedInkIDs.clear();
  regionCache.clear();
}

function sync() {
  if (!enabled()) {
    if (worker) stop();
    return;
  }
  if (worker || degraded) return;
  if (!globalThis.Worker || !globalThis.OffscreenCanvas ||
      !document.createElement('canvas').getContext('bitmaprenderer')) {
    fail('Worker or OffscreenCanvas unavailable');
    return;
  }
  try {
    const instance = new Worker(workerURL);
    worker = instance;
    instance.onmessage = ({ data }) => {
      if (worker === instance) receive(data);
      else closeBitmaps(data);
    };
    instance.onerror = event => {
      if (worker === instance) fail(event.message || 'Paper worker unavailable');
    };
    instance.onmessageerror = () => {
      if (worker === instance) fail('Paper worker returned an unreadable frame');
    };
    savedCreases = readCreases();
    instance.postMessage({ type: 'init', creases: savedCreases });
    invalidate();
    observer = new ResizeObserver(() => {
      if (layout && !sameLayout(layout, measure())) invalidate();
    });
    observer.observe(document.body);
    observer.observe(root);
  } catch (error) {
    fail(error);
  }
}

function receive(message) {
  switch (message.type) {
    case 'available':
      ({ limit, renderer } = message);
      schedule();
      break;
    case 'failed':
      fail(message.error);
      break;
    case 'creases':
      creaseRevision = message.creaseRevision;
      savedCreases = message.saved.anchored;
      try { sessionStorage.setItem(CREASE_SESSION_KEY, JSON.stringify(message.saved)); } catch {}
      break;
    case 'ink-cache':
      cachedInkIDs = new Set(message.ids);
      break;
    case 'need-ink':
      cachedInkIDs.delete(message.inkID);
      for (const [key, id] of inkIndex) {
        if (id === message.inkID) inkIndex.delete(key);
      }
      // The worker owns cache eviction. Retry the same live revision if it
      // no longer has the print that was indexed by the controller.
      if (capture?.revision === message.revision) capture = null;
      schedule();
      break;
    case 'discarded':
      if (capture?.revision === message.revision) capture = null;
      schedule();
      break;
    case 'ready':
      stageReady(message);
      break;
    case 'motion':
      queueFrame(message);
      break;
  }
}

function stageReady(message) {
  if (capture?.revision === message.layout.revision) capture = null;
  queueFrame(message);
  schedule();
}

function acknowledge(message) {
  if (message.type === 'motion') worker?.postMessage({ type: 'frame-consumed', frameID: message.frameID });
}

function discardFrame(message) {
  skippedFrames++;
  closeBitmaps(message);
  acknowledge(message);
}

function isCurrentFrame(message) {
  return message.layout.revision === revision &&
    (message.type !== 'motion' || message.motionEpoch === motionEpoch);
}

function queueFrame(message) {
  if (!isCurrentFrame(message)) {
    discardFrame(message);
    return;
  }
  if (pendingFrame) discardFrame(pendingFrame);
  pendingFrame = message;
  schedule();
}

function stockCoverage(layout) {
  const { key } = stockFrame(layout);
  const atlas = rest?.key === key ? rest : stockFrames.get(key);
  return atlas && { key, width: atlas.width, height: atlas.height, strips: atlas.strips.length };
}

function presentStock({ layout, stock, bitmap }) {
  let atlas = rest?.key === stock.key ? rest : stockFrames.get(stock.key);
  if (bitmap) {
    if (stock.start === 0) {
      detachStock();
      const node = document.createElement('div');
      node.id = 'wt-paper-stock';
      node.setAttribute('aria-hidden', 'true');
      atlas = { key: stock.key, width: stock.width, node, strips: [] };
    }
    const strip = layer(atlas.strips.length ? null : 'wt-paper-resident');
    strip.canvas.className = 'wt-paper-stock-strip';
    const cssPerPixel = layout.height / stock.height;
    paint(strip, bitmap, layout.width, (stock.height - stock.start) * cssPerPixel);
    strip.canvas.style.top = stock.start * cssPerPixel + 'px';
    atlas.strips.push(strip);
    atlas.node.append(strip.node);
    atlas.height = stock.height;
    stockFrames.put(stock.key, atlas, atlas.width * atlas.height * 4);
  }
  if (rest !== atlas) {
    detachStock();
    rest = atlas;
    document.body.append(rest.node);
  }
  // Keep every source strip at its own CSS dimensions. Only the viewport
  // changes height, so Safari/Firefox never see a cropped backing store.
  const clipHeight = Math.min(layout.height, rest.height * layout.height / stock.height);
  if (rest.logicalHeight !== clipHeight) {
    rest.logicalHeight = clipHeight;
    rest.node.style.setProperty('--wt-paper-stock-height', clipHeight + 'px');
  }
}

function presentLight(frames, layout) {
  const previous = lighting;
  const next = new Map();
  const created = [];
  for (const { key, plates, bounds } of frames) {
    if (!plates) {
      next.set(key, availableLight(key));
      continue;
    }
    const { x, y, w, h, width, height } = bounds;
    const sx = layout.width / width, sy = layout.height / height;
    const node = document.createElement('div');
    const planes = plates.map((bitmap, index) => {
      const plane = layer();
      plane.canvas.className = index === 0 ? 'wt-paper-shadow' : 'wt-paper-highlight';
      paint(plane, bitmap, w * sx, h * sy);
      plane.canvas.style.left = x * sx + 'px';
      plane.canvas.style.top = y * sy + 'px';
      node.append(plane.node);
      return plane;
    });
    const patch = { key, node, planes };
    next.set(key, patch);
    created.push([patch, w * h * 8]);
  }
  lighting = next;
  for (const [patch, bytes] of created) lightFrames.put(patch.key, patch, bytes);
  for (const frame of previous.values()) {
    if (next.get(frame.key) !== frame && !lightFrames.owns(frame.key, frame)) disposeLighting(frame);
  }
  if (!next.size) {
    lightLayer?.remove();
    lightLayer = null;
    return;
  }
  if (!lightLayer) {
    lightLayer = document.createElement('div');
    lightLayer.id = 'wt-paper-light';
    lightLayer.setAttribute('aria-hidden', 'true');
    document.body.append(lightLayer);
  }
  const patches = [...next.values()];
  if (lightLayer.children.length === patches.length &&
      patches.every((patch, i) => lightLayer.children[i] === patch.node)) return;
  for (const [i, patch] of patches.entries()) for (const plane of patch.planes) {
    if (i === 0) plane.canvas.id = plane.canvas.className;
    else plane.canvas.removeAttribute('id');
  }
  lightLayer.replaceChildren(...patches.map(patch => patch.node));
}

function presentCachedBackdrop(layout) {
  const stock = stockFrame(layout);
  const lights = creaseLightFrames(layout, savedCreases, creaseRevision);
  if (stockCoverage(layout)) presentStock({ layout, stock });
  else detachStock();
  presentLight(lights.filter(frame => availableLight(frame.key)), layout);
  root.classList.toggle('wt-paper-flat', !!rest || lighting.size > 0);
}

function present(message) {
  const { layout: frameLayout, bitmap } = message;
  const height = message.folding ? frameLayout.viewportHeight : frameLayout.height;
  const expectedStock = stockFrame(frameLayout);
  const coverage = !message.folding && stockCoverage(frameLayout);
  const stock = message.stock;
  const validStock = message.folding
    ? bitmap?.width === expectedStock.width && bitmap?.height === Math.floor(height * frameLayout.scale)
    : stock?.key === expectedStock.key && stock.width === expectedStock.width && stock.height === expectedStock.height &&
      (bitmap ? bitmap.width === stock.width && bitmap.height === stock.height - stock.start &&
        (stock.start === 0 || coverage?.height === stock.start) : coverage?.height >= stock.height);
  const lights = message.lights ?? [];
  const validLight = lights.every(({ key, plates, bounds }) => plates
    ? plates.length === 2 && plates.every(plane => plane.width === bounds.w && plane.height === bounds.h)
    : availableLight(key));
  if (!validLight || !validStock ||
      message.diagnostics.meshCssWidth !== frameLayout.width ||
      message.diagnostics.meshCssHeight !== frameLayout.height ||
      message.diagnostics.textureSize[0] !== Math.floor(frameLayout.width * frameLayout.scale) ||
      message.diagnostics.textureSize[1] !== Math.floor(frameLayout.height * frameLayout.scale)) {
    closeBitmaps(message);
    fail('Paper frame size mismatch');
    return;
  }
  if (!message.folding) {
    presentStock(message);
    presentLight(lights, frameLayout);
  }
  if (message.type === 'ready') {
    committed = frameLayout;
    latestMotion = message;
    workerTimes = {
      materialMs: message.materialMs, stockMs: message.stockMs, geometryMs: message.geometryMs, uploadMs: message.uploadMs,
      compileMs: message.compileMs, drawMs: message.drawMs, bitmapMs: message.transferMs,
      stockBitmapMs: message.stockBitmapMs, lightBitmapMs: message.lightBitmapMs,
      inkDecodeMs: message.inkDecodeMs,
      inkReused: message.inkReused, stockReused: !bitmap,
      stockCropped: !bitmap && coverage.height > stock.height, cropMs: 0,
      stockReadbackPixels: bitmap ? stock.width * (stock.height - stock.start) : 0,
      stockGrown: !!bitmap && stock.start > 0,
      lightReused: lights.length > 0 && lights.every(frame => !frame.plates),
      lightReadbackCount: lights.filter(frame => frame.plates).length,
      lightReusedCount: lights.filter(frame => !frame.plates).length,
    };
    showFold(false);
    root.classList.add('wt-physical-print', 'wt-paper-flat');
  } else if (!message.folding) {
    showFold(false);
    motionActive = false;
    latestMotion = message;
  } else if (Math.abs(message.scrollY - scrollY) < 1) {
    if (!fold) {
      fold = layer('wt-paper-fold');
      document.body.append(fold.node);
    }
    paint(fold, bitmap, frameLayout.width, height);
    shownScroll = message.scrollY;
    showFold(true);
    latestMotion = message;
  } else {
    bitmap.close();
  }
}

function advance() {
  scheduled = 0;
  if (!worker || !limit || !enabled() || !root.dataset.profileSnapshot) return;
  const now = measure();
  if (layout && !sameLayout(layout, now)) {
    invalidate();
    return;
  }
  if (pendingFrame) {
    const message = pendingFrame;
    pendingFrame = null;
    if (isCurrentFrame(message) && sameLayout(message.layout, now)) {
      try {
        present(message);
      } catch (error) {
        fail(error);
      } finally {
        acknowledge(message);
      }
    } else {
      discardFrame(message);
    }
  }
  if (worker && !capture && !committed) {
    presentCachedBackdrop(now);
    startCapture(now);
  }
}

async function startCapture(target) {
  const instance = worker;
  const abort = new AbortController();
  captureAbort = abort;
  layout = capture = target;
  const stale = () => instance !== worker || revision !== target.revision;
  let bitmap;
  try {
    const styleKey = captureStyleKey(document);
    const fingerprint = inkFingerprint(target, styleKey);
    let inkID = fingerprint && inkIndex.get(fingerprint);
    if (cachedInkIDs.has(inkID)) {
      inkIndex.delete(fingerprint);
      captureTimes = { captureMs: 0, capturePhases: {}, reusedCaptureRegions: 0 };
    } else {
      const result = await captureProfile(target, abort.signal, regionCache, styleKey);
      bitmap = result.bitmap;
      inkID = ++nextInkID;
      captureTimes = { captureMs: result.captureMs, capturePhases: result.times, reusedCaptureRegions: result.reusedRegions };
    }
    if (stale()) {
      throw new DOMException('Outdated paper capture', 'AbortError');
    }
    if (fingerprint) {
      inkIndex.set(fingerprint, inkID);
      if (inkIndex.size > 12) inkIndex.delete(inkIndex.keys().next().value);
    }
    instance.postMessage({ type: 'prepare', layout: target, bitmap, inkID: fingerprint ? inkID : null,
      stock: stockCoverage(target), lightKeys: [...new Set([...lightFrames.keys(), ...lighting.keys()])] }, bitmap ? [bitmap] : []);
  } catch (error) {
    bitmap?.close();
    if (capture === target) capture = null;
    if (stale() || error.name === 'AbortError') schedule();
    else fail(error);
  } finally {
    if (captureAbort === abort) captureAbort = null;
  }
}

function send(type, fields = {}) {
  if (worker && committed) worker.postMessage({ type, revision, motionEpoch, ...fields });
}

function candidate(event) {
  if (!committed || event.button !== 0 || event.pointerType === 'touch' ||
      event.isPrimary === false || event.target?.closest?.(controls) ||
      document.querySelector('dialog[open], [role=dialog]:not([hidden])')) return null;
  const { clientX: x, clientY: y } = event;
  const { width, height } = committed;
  const documentY = scrollY + y;
  if (x < 0 || x > width || documentY < 0 || documentY > height) return null;
  const contact = { x, y, u: x / width, v: 1 - documentY / height };
  if (latestMotion?.diagnostics.phase === 'latched') return contact;
  return Math.min(x, width - x) < Math.min(84, width * .075) &&
    Math.min(documentY, height - documentY) < 84 ? contact : null;
}

document.addEventListener('pointerdown', event => {
  if (drag) return;
  const contact = candidate(event);
  if (contact) {
    drag = { id: event.pointerId, contact, time: event.timeStamp, pending: true };
    send('warm-ink');
  }
}, true);

document.addEventListener('pointermove', event => {
  if (drag?.id !== event.pointerId || !(event.buttons & 1) || event.timeStamp <= drag.time) return;
  const dx = event.clientX - drag.contact.x, dy = event.clientY - drag.contact.y;
  if (drag.pending) {
    const travel = Math.hypot(dx, dy);
    if (travel < 12 || (event.timeStamp - drag.time < 120 && travel > Math.min(450, committed.width * .42))) return;
    drag.pending = false;
    motionActive = true;
    send('grab', { contact: drag.contact, time: drag.time, scrollY });
    document.getSelection()?.removeAllRanges();
  }
  send('drag', { dx: dx * .0085, dy: -dy * .0085, time: event.timeStamp });
  event.preventDefault();
}, true);

function release(event) {
  if (!drag || (event && drag.id !== event.pointerId)) return;
  if (!drag.pending) send('release');
  drag = null;
}

for (const event of ['pointerup', 'pointercancel']) document.addEventListener(event, release, true);
window.addEventListener('blur', () => release());
function restoreNativeControls() {
  if (!motionActive && !drag) return;
  motionActive = false;
  drag = null;
  motionEpoch++;
  showFold(false);
  send('reset');
  if (pendingFrame?.type === 'motion') {
    discardFrame(pendingFrame);
    pendingFrame = null;
  }
}
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' || event.key === 'Tab') restoreNativeControls();
}, true);
document.addEventListener('focusin', event => {
  if (event.target.closest?.(controls)) restoreNativeControls();
}, true);
document.addEventListener('click', event => {
  if (event.target.closest?.(controls)) restoreNativeControls();
}, true);
window.addEventListener('scroll', () => {
  if (foldShown && Math.abs(shownScroll - scrollY) >= 1) showFold(false);
  if (motionActive) send('scroll', { scrollY });
}, { passive: true });

document.addEventListener('scroll', event => {
  if (event.target.id === 'heat-scroll') invalidate();
}, { capture: true, passive: true });
window.addEventListener('wt:profile-render', () => {
  if (enabled() && worker) invalidate();
  else sync();
});
window.addEventListener('resize', () => {
  if (layout && !sameLayout(layout, measure())) invalidate();
  sync();
}, { passive: true });
new MutationObserver(() => {
  if (worker && enabled() && layout && !sameLayout(layout, measure())) invalidate();
  else sync();
}).observe(root, { attributeFilter: ['data-activity-palette', 'data-theme', 'lang'] });
document.addEventListener('toggle', event => {
  if (event.target.tagName === 'DETAILS') invalidate();
}, true);
document.addEventListener('visibilitychange', () => {
  worker?.postMessage({ type: 'visibility', visible: !document.hidden });
  if (!document.hidden) schedule();
});
function trackDpr() {
  dprQuery?.removeEventListener('change', trackDpr);
  dprQuery = matchMedia(`(resolution: ${devicePixelRatio}dppx)`);
  dprQuery.addEventListener('change', trackDpr, { once: true });
  if (layout && !sameLayout(layout, measure())) invalidate();
}
trackDpr();
for (const query of [fine, reduced, small, darkScheme]) query.addEventListener('change', () => {
  if (layout && enabled() && !sameLayout(layout, measure())) invalidate();
  else sync();
});

window.__WT_PAPER3D__ = {
  getDiagnostics: () => ({
    ready: !!worker, active: !!worker, backend: renderer,
    state: degraded ? 'degraded' : foldShown ? 'folding' : committed ? 'committed' : worker ? 'preparing' : 'native',
    physicalMeshResident: !!committed, inkVisibleAtRest: !!committed && !foldShown,
    presenting: committed ? 'UV-printed resident D4 mesh' : 'Native report',
    folding: foldShown, snapshotReady: !!committed, captureRevision: revision,
    readyRevision: committed?.revision ?? -1, documentKey: committed?.documentKey,
    effectiveInkScale: committed?.scale, sheetCssWidth: committed?.width,
    sheetCssHeight: committed?.height, viewportWidth: committed?.width,
    viewportHeight: committed?.viewportHeight, cameraScrollY: scrollY,
    flapPoints: latestMotion?.flapPoints ?? [], skippedFrames, error: lastError ?? null,
    ...latestMotion?.diagnostics, ...captureTimes, ...workerTimes,
    stockCacheBytes: stockFrames.bytes, stockAtlasPixels: rest && [rest.width, rest.height], stockAtlasStrips: rest?.strips.length ?? 0, lightCacheBytes: lightFrames.bytes,
    creaseLighting: lighting.size > 0 && !foldShown && root.classList.contains('wt-paper-flat'),
    lastInkUpdateMs: workerTimes?.materialMs,
  }),
  refresh: sync,
};
sync();
