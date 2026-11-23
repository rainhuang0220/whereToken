import assert from 'node:assert/strict';
import test from 'node:test';
import { compositeSurfaceMaps, prepareOpaquePrint, prepareStockMaps, tintStock } from '../src/paper/surface-maps.js';

test('the color-independent stock reproduces the original white-print textures', () => {
  const width = 37, height = 41;
  const scan = { width, height, data: Uint8ClampedArray.from({ length: width * height * 4 }, (_, i) => (i * 7291) % 251) };
  const blurred = { ...scan, data: scan.data.map((value, i) => (value + (i % 11) * 4) % 256) };
  const print = { ...scan, data: new Uint8ClampedArray(width * height * 4).fill(255) };
  const stock = prepareStockMaps(scan, blurred, width, height, .60, .97);
  const optical = prepareOpaquePrint(print);
  for (const rgb of [[247, 246, 241], [17, 16, 15], [255, 0, 119]]) {
    const params = {
      fiber: .60, roughness: .97, absorption: 1.13, pinholes: 0, spread: .50, mottle: .25,
      opaquePrint: true, paperColor: rgb.map(value => value / 255), ...optical,
    };
    const baseline = compositeSurfaceMaps({ print, scan, blurred, width, height, params });
    assert.deepEqual(tintStock(stock, rgb), baseline.albedo);
    assert.deepEqual(stock.normal, baseline.normal);
    assert.deepEqual(stock.roughness, baseline.roughness);
  }
});
