export function stockFrameKey({ width, height, scale, rgb }) {
  return JSON.stringify([width, height, scale, rgb]);
}

export function lightFrameKey(stockKey, creaseRevision) {
  return JSON.stringify([stockKey, creaseRevision]);
}
