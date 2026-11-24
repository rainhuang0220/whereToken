export const CREASE_SESSION_KEY = 'wheretoken.newsprint.creases.v1';
export const CSS_PIXELS_PER_UNIT = 100;

const MAX_CREASES = 9;
const finite = Number.isFinite;
const point = p => p && finite(p.x) && finite(p.y);

// The grab defines the normal edge. Along an edge, use the nearer end at
// grab time, keeping the signed line and its material phase fixed thereafter.
export function creaseAnchor(origin, width, height, sx, sy) {
  const right = sx > 0 || (sx === 0 && origin.x > 0);
  const top = sy > 0 || (sy === 0 && origin.y >= 0);
  return {
    x: right ? 'right' : 'left',
    y: top ? 'top' : 'bottom',
    dx: (width / 2 + (right ? -origin.x : origin.x)) * CSS_PIXELS_PER_UNIT,
    dy: (height / 2 + (top ? -origin.y : origin.y)) * CSS_PIXELS_PER_UNIT,
  };
}

function validAnchor(anchor) {
  return anchor && ['left', 'right'].includes(anchor.x) && ['top', 'bottom'].includes(anchor.y) &&
    finite(anchor.dx) && finite(anchor.dy);
}

function recordFor({ hinge: h, strength, band, times }, width, height) {
  const vertical = Math.abs(h.nx) >= Math.abs(h.ny);
  const sx = h.kind === 'corner' || vertical ? Math.sign(h.nx) : 0;
  const sy = h.kind === 'corner' || !vertical ? Math.sign(h.ny) : 0;
  const direction = Math.sign((h.end.x - h.start.x) * -h.ny + (h.end.y - h.start.y) * h.nx) || 1;
  return {
    anchor: validAnchor(h.anchor) ? h.anchor : creaseAnchor(h.origin, width, height, sx, sy),
    kind: h.kind, nx: h.nx, ny: h.ny, depth: h.depth, strength, band, times,
    direction,
    fiberStart: direction * ((h.start.x - h.origin.x) * -h.ny + (h.start.y - h.origin.y) * h.nx),
    fiberLength: h.length,
  };
}

function normalizeCreases(stored) {
  if (!Array.isArray(stored?.memories) || stored.memories.length > MAX_CREASES) return null;
  const valid = m => m && finite(m.strength) && m.strength >= 0 && m.strength <= 1 && finite(m.band);
  const line = h => h && ['corner', 'edge'].includes(h.kind) && [h.nx, h.ny, h.depth].every(finite) &&
    Math.hypot(h.nx, h.ny) > 0;
  if (stored.version === 2) {
    return stored.memories.every(m => valid(m) && line(m) && validAnchor(m.anchor) &&
      Math.abs(m.direction) === 1 && finite(m.fiberStart) && finite(m.fiberLength) && m.fiberLength > 0) ? stored : null;
  }
  if ((stored.version !== undefined && stored.version !== 1) || !finite(stored.width) || !finite(stored.height) ||
      stored.width <= 0 || stored.height <= 0) return null;
  const { width, height, memories } = stored;
  if (!memories.every(m => {
    const h = m?.hinge;
    return valid(m) && line(h) && point(h.start) && point(h.end) && point(h.origin) && finite(h.length) && h.length > 0;
  })) return null;
  return { version: 2, memories: memories.map(m => recordFor(m, width, height)) };
}

export function readCreases() {
  try {
    const value = sessionStorage.getItem(CREASE_SESSION_KEY);
    if (!value || value.length > 12000) return null;
    const stored = JSON.parse(value);
    return normalizeCreases(stored?.anchored) ?? normalizeCreases(stored);
  } catch {
    return null;
  }
}

export function restoreCreases(saved, sheet, { refresh = true } = {}) {
  const records = normalizeCreases(saved)?.memories;
  if (!records?.length) return;
  sheet.memories = [];
  for (const record of records) {
    const { anchor, nx, ny, depth, direction, kind, fiberStart, fiberLength } = record;
    const origin = {
      x: (anchor.x === 'right' ? 1 : -1) * (sheet.width / 2 - anchor.dx / CSS_PIXELS_PER_UNIT),
      y: (anchor.y === 'top' ? 1 : -1) * (sheet.height / 2 - anchor.dy / CSS_PIXELS_PER_UNIT),
    };
    const cut = sheet.lineIntersections(nx, ny, depth, origin);
    // A line entirely off the current paper still exists in the session.
    if (!cut) continue;
    const { length } = cut;
    let { start, end } = cut;
    if (((end.x - start.x) * -ny + (end.y - start.y) * nx) * direction < 0) {
      [start, end] = [end, start];
    }
    const hinge = { start, end, length, origin, nx, ny, depth, kind, anchor,
      span: length, fullSpan: true, alongShift: 0, direction, fiberStart, fiberLength };
    sheet.memories.push({ hinge, strength: record.strength, band: record.band, times: record.times, record });
  }
  if (refresh && sheet.memories.length) sheet.refreshRestingGeometry();
}

export function storeCreases(sheet, before, saved) {
  const previous = sheet.memories.at(-1);
  if (!previous || before.includes(previous)) return saved;
  const removed = new Set(before.filter(m => !sheet.memories.includes(m)).map(m => m.record));
  previous.record = recordFor(previous, sheet.width, sheet.height);
  const memories = (saved?.memories ?? []).filter(m => !removed.has(m));
  memories.push(previous.record);
  const retained = memories.slice(-MAX_CREASES);
  const visible = sheet.memories.filter(m => retained.includes(m.record));
  if (visible.length !== sheet.memories.length) {
    sheet.memories = visible;
    sheet.refreshRestingGeometry();
  }
  return { version: 2, memories: retained };
}

export function creasePositions(sheet) {
  const css = p => ({
    x: (p.x + sheet.width / 2) * CSS_PIXELS_PER_UNIT,
    y: (sheet.height / 2 - p.y) * CSS_PIXELS_PER_UNIT,
  });
  return sheet.memories.map(({ hinge: h, strength }) => ({
    anchor: h.anchor, origin: css(h.origin), start: css(h.start), end: css(h.end),
    normal: [h.nx, h.ny], depth: h.depth * CSS_PIXELS_PER_UNIT, strength,
  }));
}
