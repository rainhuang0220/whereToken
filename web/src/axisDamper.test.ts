import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { damperIndex, damperStep, damperTabs } from './axisDamper'
import type { SliceView } from './types'

function row(partial: Partial<SliceView> & Pick<SliceView, 'id' | 'label' | 'quality'>): SliceView {
  return {
    miss: 0,
    cache_read: 0,
    cache_create: 0,
    output: 0,
    total: 0,
    miss_m: '0.00 M',
    cache_read_m: '0.00 M',
    cache_create_m: '0.00 M',
    output_m: '0.00 M',
    total_m: '0.00 M',
    hit_rate: null,
    hit_rate_text: '—',
    requests: 0,
    user_turns: 0,
    ...partial,
  }
}

describe('damperTabs', () => {
  it('skips absent tools and vendors so they are not a tab stop', () => {
    const tabs = damperTabs(
      [
        row({ id: 'kimi', label: 'Kimi Code', quality: 'authoritative' }),
        row({ id: 'cursor', label: 'Cursor', quality: 'absent' }),
      ],
      [row({ id: 'unknown', label: '未知厂家', quality: 'absent' })],
    )
    expect(tabs.map((t) => t.id)).toEqual(['all', 'kimi'])
  })
})

describe('damperStep', () => {
  it('moves along the tablist with arrows', () => {
    expect(damperStep(0, 'ArrowRight', 3)).toBe(1)
    expect(damperStep(2, 'ArrowRight', 3)).toBe(2)
    expect(damperStep(1, 'ArrowLeft', 3)).toBe(0)
    expect(damperStep(2, 'Home', 3)).toBe(0)
    expect(damperStep(0, 'End', 3)).toBe(2)
  })
})

describe('damperIndex', () => {
  it('finds the selected tab', () => {
    const tabs = damperTabs([row({ id: 'kimi', label: 'Kimi Code', quality: 'authoritative' })], [])
    expect(damperIndex(tabs, { kind: 'source', id: 'kimi' })).toBe(1)
    expect(damperIndex(tabs, { kind: 'source', id: 'missing' })).toBe(0)
  })
})

describe('AxisDamper.vue', () => {
  it('uses the tab helpers instead of a bare tablist of every row', () => {
    const vue = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'components', 'AxisDamper.vue'), 'utf8')
    expect(vue).toContain('damperTabs')
    expect(vue).toContain('damperStep')
    expect(vue).toContain('tabindex')
  })
})
