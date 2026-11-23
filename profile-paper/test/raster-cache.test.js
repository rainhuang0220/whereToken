import assert from 'node:assert/strict';
import test from 'node:test';
import { RasterCache } from '../src/paper/raster-cache.js';

test('evicts least-recently used resources and does not retain oversized allocations', () => {
  const disposed = [];
  const cache = new RasterCache(10, 3, resource => disposed.push(resource));
  cache.put('a', 'A', 4);
  cache.put('b', 'B', 4);
  assert.equal(cache.get('a'), 'A');
  assert.equal(cache.put('c', 'C', 4), true);
  assert.deepEqual(disposed, ['B']);
  assert.equal(cache.put('large', 'outside cache', 12), false);
  assert.equal(cache.bytes, 8);
  cache.clear();
  assert.equal(cache.bytes, 0);
  assert.deepEqual(disposed, ['B', 'A', 'C']);
});
