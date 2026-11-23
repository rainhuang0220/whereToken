export const CREASE_SESSION_KEY = 'wheretoken.newsprint.creases.v1';

export function readCreases() {
  try {
    const value = sessionStorage.getItem(CREASE_SESSION_KEY);
    if (!value || value.length > 12000) return null;
    const stored = JSON.parse(value);
    if (!Number.isFinite(stored.width) || !Number.isFinite(stored.height) ||
      stored.width <= 0 || stored.height <= 0 ||
      !Array.isArray(stored.memories) || stored.memories.length > 9) return null;
    const point = p => p && Number.isFinite(p.x) && Number.isFinite(p.y);
    return stored.memories.every(m => {
      const h = m?.hinge;
      return h && ['corner', 'edge'].includes(h.kind) && point(h.start) && point(h.end) &&
        point(h.origin) && [h.nx, h.ny, h.depth, h.length, m.strength, m.band].every(Number.isFinite) &&
        Math.hypot(h.nx, h.ny) > 0 && h.length > 0 && m.strength >= 0 && m.strength <= 1;
    }) ? stored : null;
  } catch {
    return null;
  }
}

export function restoreCreases(saved, sheet) {
  if (!saved?.memories.length) return;
  const sx = sheet.width / saved.width;
  const sy = sheet.height / saved.height;
  const point = p => ({ x: p.x * sx, y: p.y * sy });
  sheet.memories = saved.memories.map(memory => {
    const h = memory.hinge;
    const scale = Math.hypot(h.nx / sx, h.ny / sy);
    const start = point(h.start), end = point(h.end);
    return {
      ...memory,
      hinge: {
        ...h, origin: point(h.origin), start, end,
        nx: h.nx / sx / scale, ny: h.ny / sy / scale, depth: h.depth / scale,
        length: Math.hypot(end.x - start.x, end.y - start.y),
        ...(h.span ? { span: h.span * Math.sqrt(sx * sy) } : {}),
      },
    };
  });
  sheet.refreshRestingGeometry();
}
