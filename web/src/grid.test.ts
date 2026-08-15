import { describe, expect, it } from 'vitest'
import { layoutCells } from './grid'
import type { Day } from './types'

const lit: Day = {
  date: '2026-08-14',
  miss: 1,
  cache_read: 0,
  cache_create: 0,
  output: 0,
  total: 1,
  miss_m: '0.0000 M',
  cache_read_m: '0.00 M',
  cache_create_m: '0.00 M',
  output_m: '0.00 M',
  total_m: '0.0000 M',
  level: 2,
}

describe('layoutCells', () => {
  it('marks missing in-window days empty and dates after today future', () => {
    const cells = layoutCells({
      windowFrom: '2026-08-10',
      windowTo: '2026-08-16',
      today: '2026-08-15',
      weekStart: 'monday',
      days: [lit],
    })
    const byDate = Object.fromEntries(cells.map((c) => [c.date, c.kind]))
    expect(byDate['2026-08-14']).toBe('lit')
    expect(byDate['2026-08-13']).toBe('empty')
    expect(byDate['2026-08-16']).toBe('future')
  })

  it('pads the current week with future days after today', () => {
    const cells = layoutCells({
      windowFrom: '2026-08-10',
      windowTo: '2026-08-15',
      today: '2026-08-15',
      weekStart: 'monday',
      days: [],
    })
    expect(cells.at(-1)?.date).toBe('2026-08-16')
    expect(cells.at(-1)?.kind).toBe('future')
  })
})
