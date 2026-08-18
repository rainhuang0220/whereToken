import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { isRowActivateKey, rowIsSelectable } from './sliceTable'

describe('slice table activation', () => {
  it('refuses absent rows and treats Enter/Space as activate', () => {
    expect(rowIsSelectable('absent')).toBe(false)
    expect(rowIsSelectable('degraded')).toBe(true)
    expect(rowIsSelectable('authoritative')).toBe(true)
    expect(isRowActivateKey('Enter')).toBe(true)
    expect(isRowActivateKey(' ')).toBe(true)
    expect(isRowActivateKey('Tab')).toBe(false)
  })

  it('is the function SliceTable.vue calls on click and keydown', () => {
    const vue = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'components', 'SliceTable.vue'), 'utf8')
    expect(vue).toContain('rowIsSelectable')
    expect(vue).toContain('isRowActivateKey')
    expect(vue).toContain('qualityCaption')
    expect(vue).toContain('tabindex')
  })
})
