import assert from 'node:assert/strict';
import test from 'node:test';
import { PaperFoldSheet } from '../src/scene/paper-foldsheet.js';
import { restoreCreases, storeCreases } from '../src/scene/crease-memory.js';

const makeSheet = (width, height) => new PaperFoldSheet({ width: width / 100, height: height / 100 });
const horizontal = (sheet, x) => (x + sheet.width / 2) * 100;
const vertical = (sheet, y) => (sheet.height / 2 - y) * 100;

function crossings(sheet, hinge, x, y) {
  const atX = hinge.origin.y - (hinge.depth + (x - hinge.origin.x) * hinge.nx) / hinge.ny;
  const atY = hinge.origin.x - (hinge.depth + (y - hinge.origin.y) * hinge.ny) / hinge.nx;
  return { x: horizontal(sheet, atY), y: vertical(sheet, atX) };
}

function assertCut(sheet, hinge) {
  for (const point of [hinge.start, hinge.end]) {
    const dx = sheet.width / 2 - Math.abs(point.x);
    const dy = sheet.height / 2 - Math.abs(point.y);
    assert.ok(Math.min(dx, dy) >= -1e-6 && Math.min(dx, dy) < 1e-6);
    assert.ok(Math.abs(sheet.q(point, hinge)) < 1e-6);
  }
  assert.equal(hinge.length, Math.hypot(hinge.end.x - hinge.start.x, hinge.end.y - hinge.start.y));
}

test('legacy top and bottom corner scars retain their edge distances through height changes', () => {
  const original = makeSheet(1440, 1635);
  const offsets = [1, -1].map(sy => {
    const hinge = original.computeHinge(original.getCandidate(.01, sy > 0 ? .99 : .01), 1.1, -sy * 1.05);
    return { hinge, strength: .7, band: .2 };
  });
  const saved = { width: original.width, height: original.height, memories: offsets.map(memory => {
    const hinge = { ...memory.hinge };
    delete hinge.anchor;
    return { ...memory, hinge };
  }) };
  for (const height of [1286, 1635]) {
    const sheet = makeSheet(1440, height);
    restoreCreases(saved, sheet);
    assert.equal(sheet.memories.length, 2);
    sheet.memories.forEach(({ hinge }, i) => {
      const old = offsets[i].hinge;
      const x = -6.5;
      const before = crossings(original, old, x, 0).y;
      const after = crossings(sheet, hinge, x, 0).y;
      const drift = i === 0 ? after - before : (height - after) - (1635 - before);
      assert.ok(Math.abs(drift) <= 2, `height ${height}, crease ${i}, drift ${drift.toFixed(2)} CSS px`);
      assertCut(sheet, hinge);
    });
  }
});

test('tilted left and right side scars keep the grab point’s physical edge offsets and reclip', () => {
  const original = makeSheet(1440, 1635);
  const memories = [1, -1].map(sx => ({
    hinge: original.computeHinge(original.getCandidate(sx > 0 ? .99 : .01, .72), -sx * 2, 1.3),
    strength: .7, band: .2,
  }));
  const sheet = makeSheet(1080, 1286);
  restoreCreases({ width: original.width, height: original.height, memories }, sheet);
  const y = 4.45 - (1635 - 1286) / 200;
  sheet.memories.forEach(({ hinge }, i) => {
    const old = memories[i].hinge;
    const before = crossings(original, old, 0, 4.45).x;
    const after = crossings(sheet, hinge, 0, y).x;
    const drift = i === 0 ? (1080 - after) - (1440 - before) : after - before;
    assert.ok(Math.abs(drift) <= 2, `side ${i} drift ${drift.toFixed(2)} CSS px`);
    assert.equal(hinge.depth, old.depth);
    assert.equal(hinge.nx, old.nx);
    assert.equal(hinge.ny, old.ny);
    assertCut(sheet, hinge);
  });
});

test('an off-sheet crease reappears when the document grows after a second recorded fold', () => {
  const tall = makeSheet(1440, 1635);
  tall.hinge = tall.computeHinge(tall.getCandidate(.01, .99), 8, -11);
  tall.peakTarget = 2.9;
  tall.recordMemory(true);
  const initial = storeCreases(tall, [], null);
  assert.equal(initial.version, 2);
  assert.deepEqual(initial.memories[0].anchor, { x: 'left', y: 'top', dx: 0, dy: 0 });
  const short = makeSheet(400, 400);
  restoreCreases(initial, short);
  assert.equal(short.memories.length, 0);
  short.hinge = short.computeHinge(short.getCandidate(.99, .01), -1.2, 1.2);
  short.peakTarget = 1.1;
  short.recordMemory(false);
  const both = storeCreases(short, [], initial);
  const regrown = makeSheet(1440, 1635);
  restoreCreases(both, regrown);
  assert.equal(regrown.memories.length, 2);
  assert.equal(regrown.memories[0].strength, tall.memories[0].strength);
  assert.deepEqual(regrown.memories[0].hinge.start, tall.memories[0].hinge.start);
  assert.deepEqual(regrown.memories[0].hinge.end, tall.memories[0].hinge.end);
  assert.equal(regrown.memories[1].hinge.anchor.y, 'bottom');
  for (const { hinge } of regrown.memories) assertCut(regrown, hinge);
});

test('the nine-crease limit removes the same scar from the visible sheet and stored history', () => {
  const tall = makeSheet(1440, 1635);
  const corner = tall.getCandidate(.01, .99);
  const makeRecord = (dx, dy) => {
    tall.hinge = tall.computeHinge(corner, dx, dy);
    tall.peakTarget = 2.1;
    tall.recordMemory();
    return storeCreases(tall, [], null).memories[0];
  };
  const first = makeRecord(1, -1);
  const offSheet = makeRecord(8, -11);
  const saved = { version: 2, memories: [first, offSheet, ...Array.from({ length: 7 }, () => ({ ...first }))] };
  const short = makeSheet(400, 400);
  restoreCreases(saved, short, { refresh: false });
  assert.equal(short.memories.length, 8);
  const before = short.memories.slice();
  short.hinge = short.computeHinge(short.getCandidate(.99, .01), -1.2, 1.2);
  short.peakTarget = 1.1;
  short.recordMemory();
  const updated = storeCreases(short, before, saved);
  assert.equal(updated.memories.length, 9);
  assert.equal(updated.memories[0], offSheet);
  assert.equal(short.memories.length, 8);
  assert.ok(short.memories.every(m => updated.memories.includes(m.record)));
});
