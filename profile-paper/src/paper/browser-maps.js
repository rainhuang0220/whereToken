import { evaluatePaperField } from './height-field.js';
import { compositeSurfaceMaps, prepareOpaquePrint } from './surface-maps.js';

const materialBaseCache = new WeakMap();

export function loadLocalImage(src) {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error(`Local material image failed: ${src}`));
    image.src = src;
  });
}

function drawMap(image, width, height, blur = 0) {
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  context.filter = blur > 0 ? `blur(${blur}px)` : 'none';
  context.drawImage(image, 0, 0, width, height);
  return canvas;
}

export function toCanvas(map) {
  const canvas = document.createElement('canvas');
  canvas.width = map.width;
  canvas.height = map.height;
  const context = canvas.getContext('2d');
  const image = new ImageData(new Uint8ClampedArray(map.data), map.width, map.height);
  context.putImageData(image, 0, 0);
  return canvas;
}

export function getMaterialBase(printCanvas, scanImage) {
  const width = printCanvas.width;
  const height = printCanvas.height;
  let base = materialBaseCache.get(printCanvas);
  if (!base || base.scanImage !== scanImage) {
    const scan = drawMap(scanImage, width, height).getContext('2d', { willReadFrequently: true }).getImageData(0, 0, width, height);
    const blurred = drawMap(scanImage, width, height, 9).getContext('2d', { willReadFrequently: true }).getImageData(0, 0, width, height);
    const print = printCanvas.getContext('2d', { willReadFrequently: true }).getImageData(0, 0, width, height);
    base = { scanImage, width, height, print, scan, blurred, optical: prepareOpaquePrint(print) };
    materialBaseCache.set(printCanvas, base);
  }
  return base;
}

export function buildMaterialCanvases(printCanvas, scanImage, params) {
  const base = getMaterialBase(printCanvas, scanImage);
  const maps = compositeSurfaceMaps({ print: base.print, scan: base.scan, blurred: base.blurred, width: base.width, height: base.height, params: { ...params, ...base.optical } });
  return {
    albedo: toCanvas(maps.albedo),
    ink: toCanvas(maps.ink),
    roughness: toCanvas(maps.roughness),
    normal: toCanvas(maps.normal),
  };
}

export function buildHeightCanvas(params, width = 768, height = 576) {
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext('2d');
  const image = context.createImageData(width, height);
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const field = evaluatePaperField(x / (width - 1), y / (height - 1), params);
      const value = Math.round(Math.min(1, Math.max(0, 0.5 + field.height / 0.32)) * 255);
      const offset = (y * width + x) * 4;
      image.data[offset] = value;
      image.data[offset + 1] = value;
      image.data[offset + 2] = value;
      image.data[offset + 3] = 255;
    }
  }
  context.putImageData(image, 0, 0);
  return canvas;
}
