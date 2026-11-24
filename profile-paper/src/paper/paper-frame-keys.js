const STOCK_MATERIAL_VERSION = 1;
export const MAX_STOCK_STRIPS = 8;

export function stockFrameKey({ width, height, scale, rgb }) {
  // Include the actual vertical pitch after flooring the drawing buffer.
  // Fractional scales can share a raster only when their pitches match.
  return JSON.stringify([STOCK_MATERIAL_VERSION, width, scale, Math.floor(height * scale) / height, rgb]);
}

export function stockFrame(layout) {
  return {
    key: stockFrameKey(layout),
    width: Math.floor(layout.width * layout.scale),
    height: Math.floor(layout.height * layout.scale),
  };
}
