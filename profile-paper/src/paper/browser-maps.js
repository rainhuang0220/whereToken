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

// Keep paper fibers at a fixed CSS-pixel scale even as the document expands.
// Mirrored tiles meet at identical edges; no stretched "white noise" overlay.
function drawMap(image,width,height,blur=0,scale=1){
 const canvas=document.createElement('canvas');
 canvas.width=width;canvas.height=height;
 const ctx=canvas.getContext('2d',{willReadFrequently:true});
 const sourceWidth=image.naturalWidth||image.width;
 const sourceHeight=image.naturalHeight||image.height;
 const tileW=560*scale,tileH=tileW*sourceHeight/sourceWidth;
 const cols=Math.ceil(width/tileW),rows=Math.ceil(height/tileH);
 ctx.filter=blur>0?'blur('+blur+'px)':'none';
 for(let row=0;row<rows;row++)for(let col=0;col<cols;col++){
  ctx.save();
  ctx.translate((col+(col%2===0?0:1))*tileW,(row+(row%2===0?0:1))*tileH);
  ctx.scale(col%2===0?1:-1,row%2===0?1:-1);
  ctx.drawImage(image,0,0,tileW,tileH);
  ctx.restore();
 }
 ctx.filter='none';
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

export function getMaterialBase(printCanvas, scanImage, scale = 1) {
  const width = printCanvas.width;
  const height = printCanvas.height;
  let base = materialBaseCache.get(printCanvas);
  if (!base || base.scanImage !== scanImage) {
    const scan = drawMap(scanImage, width, height, 0, scale).getContext('2d', { willReadFrequently: true }).getImageData(0, 0, width, height);
    const blurred = drawMap(scanImage, width, height, 8*scale, scale).getContext('2d', { willReadFrequently: true }).getImageData(0, 0, width, height);
    const print = printCanvas.getContext('2d', { willReadFrequently: true }).getImageData(0, 0, width, height);
    base = { scanImage, width, height, print, scan, blurred, optical: prepareOpaquePrint(print) };
    materialBaseCache.set(printCanvas, base);
  }
  return base;
}

export function buildMaterialCanvases(printCanvas, scanImage, params) {
  const base = getMaterialBase(printCanvas, scanImage, params.cssScale || 1);
  const maps = compositeSurfaceMaps({ print: base.print, scan: base.scan, blurred: base.blurred, width: base.width, height: base.height, params: { ...params, ...base.optical } });
  return {
    albedo: toCanvas(maps.albedo),
    substrate: toCanvas(maps.substrate),
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
