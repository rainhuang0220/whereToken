import assert from 'node:assert/strict';
import test from 'node:test';
import { creaseLightFrames } from '../src/paper/crease-light-frame.js';
import { stockFrameKey } from '../src/paper/paper-frame-keys.js';
import { PaperFoldSheet } from '../src/scene/paper-foldsheet.js';

const tall = { width: 1440, height: 1635, scale: 2, rgb: [247, 246, 241] };
const short = { ...tall, height: 1203 };

function memory(u, v, dx, dy) {
  const sheet = new PaperFoldSheet({ width: 14.4, height: 16.35 });
  const hinge = sheet.computeHinge(sheet.getCandidate(u, v), dx, dy);
  return { hinge, strength: .85, band: .2 };
}
const saved = (...memories) => ({ width: 14.4, height: 16.35, memories });

test('stock shares only the same paper color and exact raster pitch', () => {
  assert.equal(stockFrameKey(tall), stockFrameKey(short));
  assert.notEqual(stockFrameKey(tall), stockFrameKey({ ...short, rgb: [42, 42, 42] }));
  assert.notEqual(stockFrameKey({ ...tall, scale: 1.25 }), stockFrameKey({ ...short, scale: 1.25 }));
});

test('the same upper crease reuses a bounded light raster across all three heights', () => {
  const top = saved(memory(.01, .99, 2, -2));
  const [original] = creaseLightFrames(tall, top, 0);
  for (const height of [1286, 1203]) {
    const [next] = creaseLightFrames({ ...tall, height }, top, 0);
    assert.equal(next.key, original.key);
    assert.deepEqual([next.bounds.x, next.bounds.y, next.bounds.w, next.bounds.h],
      [original.bounds.x, original.bounds.y, original.bounds.w, original.bounds.h]);
  }
  assert.notEqual(creaseLightFrames(tall, top, 1)[0].key, original.key);
  assert.notEqual(creaseLightFrames({ ...tall, scale: 1 }, top, 0)[0].key, original.key);
});

test('moved lower and re-clipped side creases leave the disjoint upper raster reusable', () => {
  const upper = memory(.01, .99, 2, -2);
  for (const changed of [memory(.99, .01, -2, 2), memory(.99, .55, -2, 1)]) {
    const marks = saved(upper, changed);
    const [oldUpper, oldOther] = creaseLightFrames(tall, marks, 0);
    const [newUpper, newOther] = creaseLightFrames(short, marks, 0);
    assert.equal(oldUpper.key, newUpper.key);
    assert.notEqual(oldOther.key, newOther.key);
  }
});


test('overlapping crease footprints are combined for one optical calculation', () => {
  const marks = saved(memory(.01, .99, 2, -2), memory(.01, .99, 2.5, -1));
  assert.equal(creaseLightFrames(tall, marks, 0).length, 1);
});
