import { compositeInk, linearToSrgb, srgbToLinear } from './ink.js';

const clamp01 = (value) => Math.min(1, Math.max(0, value));
const byte = (value) => Math.round(clamp01(value) * 255);
const image = (width, height) => ({ data: new Uint8ClampedArray(width * height * 4), width, height });

export function mottleAt(u, v, fiber = 0, formation = 0) {
  const low = Math.sin((u * 7.3 + v * 2.1) * Math.PI * 2) * 0.012;
  const middle = Math.sin((v * 13.7 - u * 1.9) * Math.PI * 2 + 1.2) * 0.008;
  const correlated = Math.max(-0.012, Math.min(0.012, fiber * 0.018 + formation * 0.025));
  return Math.max(-0.04, Math.min(0.04, low + middle + correlated));
}

function paperHash(x, y, seed = 1) {
  let n = Math.imul(x + 151, 374761393) ^ Math.imul(y + 733, 668265263) ^ Math.imul(seed, 2246822519);
  n = Math.imul(n ^ (n >>> 13), 1274126177);
  return ((n ^ (n >>> 16)) >>> 0) / 4294967295;
}

export function inkDropout(x, y, fiber = 0, amount = 0, scale = 1) {
  if (amount <= 0) return 0;
  const dx = (paperHash(Math.floor(x / 7), Math.floor(y / 7), 7) - 0.5) * 3;
  const dy = (paperHash(Math.floor(x / 7), Math.floor(y / 7), 11) - 0.5) * 3;
  const sx = Math.floor((x + dx) / (2.1 * scale));
  const sy = Math.floor((y + dy) / (2.1 * scale));
  const patch = paperHash(sx, sy, 73);
  const fine = paperHash(x, y, 41);
  const fiberBias = Math.max(0, Math.min(.3, -fiber * 1.9));
  const clustered = patch < (.032 + fiberBias * .018) * amount && fine < .75;
  const isolated = fine < .0035 * amount;
  if (!clustered && !isolated) return 0;
  const strength = clustered ? .65 + .25 * paperHash(sx, sy, 31) : .50 + .38 * paperHash(x, y, 17);
  return strength * Math.min(1, amount);
}

export function generateNormalPixels(heights, width, height, strength) {
  const output = image(width, height);
  for (let y = 0; y < height; y++) {
    const ym = Math.max(0, y - 1);
    const yp = Math.min(height - 1, y + 1);
    for (let x = 0; x < width; x++) {
      const xm = Math.max(0, x - 1);
      const xp = Math.min(width - 1, x + 1);
      const dx = (heights[y * width + xp] - heights[y * width + xm]) * strength * 0.5;
      const dy = (heights[yp * width + x] - heights[ym * width + x]) * strength * 0.5;
      const length = Math.hypot(dx, dy, 1);
      const offset = (y * width + x) * 4;
      output.data[offset] = byte(0.5 - dx / length * 0.5);
      output.data[offset + 1] = byte(0.5 - dy / length * 0.5);
      output.data[offset + 2] = byte(0.5 + 1 / length * 0.5);
      output.data[offset + 3] = 255;
    }
  }
  return output.data;
}

function coverageAt(print, width, height, x, y) {
  const xm = Math.max(0, x - 1);
  const xp = Math.min(width - 1, x + 1);
  const ym = Math.max(0, y - 1);
  const yp = Math.min(height - 1, y + 1);
  const center = print.data[(y * width + x) * 4 + 3] / 255;
  const neighbors = [
    ym * width + xm, ym * width + x, ym * width + xp,
    y * width + xm, y * width + xp,
    yp * width + xm, yp * width + x, yp * width + xp,
  ];
  let average = 0;
  for (const index of neighbors) average += print.data[index * 4 + 3] / 255;
  return { center, average: average / neighbors.length };
}

function scalarOpticalDensity(data, offset) {
  const r = data[offset];
  const g = data[offset + 1];
  const b = data[offset + 2];
  const neutral = Math.max(r, g, b) - Math.min(r, g, b) < 45;
  const encoded = neutral ? (r * 0.2126 + g * 0.7152 + b * 0.0722) / 255 : (r + g + b) / 765;
  return -Math.log(Math.max(0.0001, srgbToLinear(encoded)));
}

export function prepareOpaquePrint(print) {
  const count = print.width * print.height;
  const opticalScalar = new Float32Array(count);
  const opticalDepth = new Float32Array(count * 3);
  for (let pixel = 0; pixel < count; pixel++) {
    const offset = pixel * 4;
    const r = print.data[offset];
    const g = print.data[offset + 1];
    const b = print.data[offset + 2];
    const neutral = Math.max(r, g, b) - Math.min(r, g, b) < 45;
    const luma = (r * 0.2126 + g * 0.7152 + b * 0.0722) / 255;
    opticalScalar[pixel] = -Math.log(Math.max(0.0001, srgbToLinear(neutral ? luma : (r + g + b) / 765)));
    for (let channel = 0; channel < 3; channel++) {
      const encoded = neutral ? luma : print.data[offset + channel] / 255;
      opticalDepth[pixel * 3 + channel] = -Math.log(Math.max(0.0001, srgbToLinear(encoded)));
    }
  }
  return { opticalScalar, opticalDepth };
}

export function compositeSurfaceMaps({ print, scan, blurred, width, height, params }) {
  const albedo = image(width, height);
  const ink = image(width, height);
  const roughness = image(width, height);
  const fiberHeight = new Float32Array(width * height);
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const pixel = y * width + x;
      const offset = pixel * 4;
      const fiber = (scan.data[offset] - blurred.data[offset]) / 255;
      const formation = (blurred.data[offset] - 128) / 255;
      fiberHeight[pixel] = fiber;
      const mottle = mottleAt(x / Math.max(1, width - 1), y / Math.max(1, height - 1), fiber, formation) * params.mottle;
      const substrateVariation = 1 + formation * 0.055 + fiber * 0.012;
      let coverage;
      let color;
      if (params.opaquePrint) {
        const centerDensity = params.opticalScalar?.[pixel] ?? scalarOpticalDensity(print.data, offset);
        const xm = Math.max(0, x - 1);
        const xp = Math.min(width - 1, x + 1);
        const ym = Math.max(0, y - 1);
        const yp = Math.min(height - 1, y + 1);
        const neighbors = [ym * width + xm, ym * width + x, ym * width + xp, y * width + xm, y * width + xp, yp * width + xm, yp * width + x, yp * width + xp];
        let averageDensity = 0;
        for (const index of neighbors) averageDensity += params.opticalScalar?.[index] ?? scalarOpticalDensity(print.data, index * 4);
        averageDensity /= neighbors.length;
        const spreadDensity = Math.max(0, averageDensity - centerDensity) * params.spread * 0.12;
        coverage = clamp01((centerDensity + spreadDensity) / 4.6);
        const densityFactor = params.absorption * Math.max(0, 1 + mottle);
        const r = print.data[offset];
        const g = print.data[offset + 1];
        const b = print.data[offset + 2];
        const neutral = Math.max(r, g, b) - Math.min(r, g, b) < 45;
        const luma = (r * 0.2126 + g * 0.7152 + b * 0.0722) / 255;
        color = [0, 1, 2].map((channel) => {
          const encoded = neutral ? luma : print.data[offset + channel] / 255;
          const opticalDepth = params.opticalDepth?.[pixel * 3 + channel] ?? -Math.log(Math.max(0.0001, srgbToLinear(encoded)));
          const transmission = Math.exp(-(opticalDepth + spreadDensity) * densityFactor);
          return srgbToLinear(params.paperColor[channel]) * substrateVariation * transmission;
        });
      } else {
        const { center, average } = coverageAt(print, width, height, x, y);
        coverage = clamp01(center + Math.max(0, average - center) * params.spread * 0.12);
        const paperLinear = params.paperColor.map((value) => srgbToLinear(value) * substrateVariation);
        const pigmentLinear = [0, 1, 2].map((channel) => srgbToLinear(print.data[offset + channel] / 255));
        color = compositeInk(paperLinear, pigmentLinear, coverage, { absorption: params.absorption, mottle });
      }
      const dropout = params.opaquePrint && coverage > .76
        ? inkDropout(x, y, fiber, params.pinholes || 0, params.pinholeScale || 1) : 0;
      const remainingInk = coverage * (1 - dropout);
      for (let channel = 0; channel < 3; channel++) {
        const paper = srgbToLinear(params.paperColor[channel]) * substrateVariation;
        const printed = color[channel] * (1 - dropout) + paper * dropout;
        albedo.data[offset + channel] = byte(linearToSrgb(printed));
        ink.data[offset + channel] = byte(remainingInk);
      }
      albedo.data[offset + 3] = 255;
      ink.data[offset + 3] = byte(remainingInk);
      const rough = Math.min(0.98, Math.max(0.72, params.roughness + formation * 0.04 + fiber * 0.012));
      roughness.data[offset] = byte(rough);
      roughness.data[offset + 1] = byte(rough);
      roughness.data[offset + 2] = byte(rough);
      roughness.data[offset + 3] = 255;
    }
  }
  const normal = { data: generateNormalPixels(fiberHeight, width, height, params.fiber * 5), width, height };
  return { albedo, ink, roughness, normal, fiberHeight };
}
