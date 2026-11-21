export const PAPER_PRESETS = Object.freeze({
  fresh: Object.freeze({ macro: 1, cockling: 0.65, crease: 0 }),
  folded: Object.freeze({ macro: 1, cockling: 0.85, crease: 1 }),
});

const clamp01 = (value) => Math.min(1, Math.max(0, value));
const smooth = (value) => value * value * (3 - 2 * value);

function hash(x, y) {
  let value = Math.imul(x, 374761393) + Math.imul(y, 668265263);
  value = Math.imul(value ^ (value >>> 13), 1274126177);
  return ((value ^ (value >>> 16)) >>> 0) / 4294967295;
}

function noise(x, y) {
  const ix = Math.floor(x);
  const iy = Math.floor(y);
  const fx = smooth(x - ix);
  const fy = smooth(y - iy);
  const a = hash(ix, iy);
  const b = hash(ix + 1, iy);
  const c = hash(ix, iy + 1);
  const d = hash(ix + 1, iy + 1);
  return ((a + (b - a) * fx) + ((c + (d - c) * fx) - (a + (b - a) * fx)) * fy) * 2 - 1;
}

function macroShape(u, v) {
  const x = (u - 0.5) * 2;
  const y = (v - 0.5) * 2;
  const edgeX = smooth(clamp01((Math.abs(x) - 0.68) / 0.32));
  const edgeY = smooth(clamp01((Math.abs(y) - 0.76) / 0.24));
  const bow = 0.038 * x * x + 0.018 * y * y + 0.012 * x * y;
  const cornerCurl = 0.055 * edgeX * edgeY * (0.65 + 0.35 * x * y);
  const freeEdge = 0.028 * edgeX * (0.5 + 0.5 * Math.sin(v * 5.3 + 0.4));
  return bow + cornerCurl + freeEdge;
}

function cockleShape(u, v) {
  const warpX = noise(u * 3.1 + 9.2, v * 3.1 - 4.7) * 0.055;
  const warpY = noise(u * 3.1 - 7.4, v * 3.1 + 8.3) * 0.055;
  const x = u + warpX;
  const y = v + warpY;
  const broad = noise(x * 5.3 + 2.8, y * 5.3 - 1.9);
  const middle = noise(x * 11.7 - 6.1, y * 11.7 + 3.4) * 0.48;
  const directional = noise(x * 18.2 + y * 3.4, y * 9.6 - x * 2.1) * 0.22;
  return 0.028 * (broad + middle + directional) / 1.7;
}

function creaseShape(u, v) {
  const distance = u - 0.5 + 0.026 * (v - 0.5) + 0.004 * Math.sin(v * Math.PI * 2);
  const ridge = Math.exp(-((distance / 0.012) ** 2));
  const shoulder = Math.exp(-((distance / 0.032) ** 2));
  const crossDistance = v - 0.69 + 0.018 * (u - 0.5);
  const cross = Math.exp(-((crossDistance / 0.015) ** 2)) * 0.32;
  return 0.072 * (ridge - 0.3 * shoulder + cross);
}

export function evaluatePaperField(u, v, params = PAPER_PRESETS.fresh) {
  const macro = (params.macro ?? 1) * macroShape(u, v);
  const cockling = (params.cockling ?? 0) * cockleShape(u, v);
  const crease = (params.crease ?? 0) * creaseShape(u, v);
  return { macro, cockling, crease, height: macro + cockling + crease };
}

export function samplePaperNormal(u, v, params = PAPER_PRESETS.fresh) {
  const epsilon = 0.0002;
  const dx = (evaluatePaperField(u + epsilon, v, params).height - evaluatePaperField(u - epsilon, v, params).height) / (2 * epsilon);
  const dy = (evaluatePaperField(u, v + epsilon, params).height - evaluatePaperField(u, v - epsilon, params).height) / (2 * epsilon);
  const length = Math.hypot(dx, dy, 1);
  return [-dx / length, -dy / length, 1 / length];
}

export function buildPaperGeometryData({ width, height, segmentsX, segmentsY, params }) {
  const vertexCount = (segmentsX + 1) * (segmentsY + 1);
  const positions = new Float32Array(vertexCount * 3);
  const uvs = new Float32Array(vertexCount * 2);
  const indices = new Uint32Array(segmentsX * segmentsY * 6);
  let minHeight = Infinity;
  let maxHeight = -Infinity;
  for (let y = 0; y <= segmentsY; y++) {
    const v = y / segmentsY;
    for (let x = 0; x <= segmentsX; x++) {
      const u = x / segmentsX;
      const vertex = y * (segmentsX + 1) + x;
      const surfaceHeight = evaluatePaperField(u, v, params).height;
      positions[vertex * 3] = (u - 0.5) * width;
      positions[vertex * 3 + 1] = (0.5 - v) * height;
      positions[vertex * 3 + 2] = surfaceHeight;
      uvs[vertex * 2] = u;
      uvs[vertex * 2 + 1] = v;
      minHeight = Math.min(minHeight, surfaceHeight);
      maxHeight = Math.max(maxHeight, surfaceHeight);
      if (x < segmentsX && y < segmentsY) {
        const offset = (y * segmentsX + x) * 6;
        const below = vertex + segmentsX + 1;
        indices[offset] = vertex;
        indices[offset + 1] = below;
        indices[offset + 2] = vertex + 1;
        indices[offset + 3] = vertex + 1;
        indices[offset + 4] = below;
        indices[offset + 5] = below + 1;
      }
    }
  }
  return { positions, uvs, indices, minHeight, maxHeight };
}
