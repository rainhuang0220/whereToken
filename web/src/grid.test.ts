import { describe, expect, it } from 'vitest'
import { layoutCells, selectDrill, selectSeries, wallCells } from './grid'
import type { Calendar, Day, SummaryPayload } from './types'

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

function day(partial: Partial<Day> & Pick<Day, 'date' | 'total' | 'total_m' | 'level'>): Day {
  return {
    miss: 0,
    cache_read: 0,
    cache_create: 0,
    output: 0,
    miss_m: '0.00 M',
    cache_read_m: '0.00 M',
    cache_create_m: '0.00 M',
    output_m: '0.00 M',
    ...partial,
  }
}

function payloadWithCalendar(cal: Calendar): SummaryPayload {
  return {
    all: {
      id: 'all',
      label: '合计',
      miss: 0,
      cache_read: 0,
      cache_create: 0,
      output: 0,
      total: 100,
      miss_m: '0.00 M',
      cache_read_m: '0.00 M',
      cache_create_m: '0.00 M',
      output_m: '0.00 M',
      total_m: '0.0001 M',
      hit_rate: null,
      hit_rate_text: '—',
      requests: 1,
      user_turns: 0,
      quality: 'authoritative',
    },
    by_source: [],
    by_vendor: [],
    by_source_vendor: [],
    calendar: cal,
    errors: [],
  }
}

describe('selectSeries / wallCells', () => {
  const cal: Calendar = {
    week_start: 'monday',
    timezone: 'Asia/Shanghai',
    window_from: '2026-08-10',
    window_to: '2026-08-15',
    all: {
      days: [day({ date: '2026-08-14', total: 100, total_m: '0.0001 M', level: 2 })],
      stats: {
        peak_date: '2026-08-14',
        peak_total: 100,
        peak_total_m: '0.0001 M',
        current_streak: 1,
        longest_streak: 1,
      },
    },
    by_source: {
      kimi: {
        days: [day({ date: '2026-08-14', total: 40, total_m: '0.0000 M', level: 2 })],
        stats: {
          peak_date: '2026-08-14',
          peak_total: 40,
          peak_total_m: '0.0000 M',
          current_streak: 1,
          longest_streak: 1,
        },
      },
    },
    by_vendor: {},
  }

  it('binds calendar.all so the wall is not an empty series when JSON has days', () => {
    const payload = payloadWithCalendar(cal)
    const series = selectSeries(payload, { kind: 'all', id: 'all' })
    expect(series.stats.peak_total_m).toBe('0.0001 M')
    expect(series.days).toHaveLength(1)
    const cells = wallCells(payload, { kind: 'all', id: 'all' }, '2026-08-15')
    expect(cells.length).toBe(7)
    expect(cells.filter((c) => c.kind === 'lit')).toHaveLength(1)
    expect(cells.filter((c) => c.kind === 'empty').length).toBeGreaterThan(0)
  })

  it('recolors from by_source when the tool axis is selected', () => {
    const series = selectSeries(payloadWithCalendar(cal), { kind: 'source', id: 'kimi' })
    expect(series.stats.peak_total).toBe(40)
  })

  it('still lays 53×7 clay bricks when calendar is missing from JSON (stale /api/summary)', () => {
    const payload = payloadWithCalendar(cal)
    const stale = { ...payload } as SummaryPayload
    delete (stale as { calendar?: Calendar }).calendar
    const series = selectSeries(stale, { kind: 'all', id: 'all' })
    expect(series.stats.peak_total_m).toBe('0.00 M')
    const cells = wallCells(stale, { kind: 'all', id: 'all' }, '2026-08-15')
    expect(cells.length).toBe(371)
    expect(cells.every((c) => c.kind !== 'lit')).toBe(true)
    expect(cells.some((c) => c.kind === 'empty')).toBe(true)
  })

  it('picks pre-aggregated drill tables without summing on the client', () => {
    const payload = payloadWithCalendar(cal)
    payload.drill = {
      all: {
        models: [
          {
            id: 'k2',
            label: 'k2',
            miss: 100,
            cache_read: 0,
            cache_create: 0,
            output: 0,
            total: 100,
            miss_m: '0.0001 M',
            cache_read_m: '0.00 M',
            cache_create_m: '0.00 M',
            output_m: '0.00 M',
            total_m: '0.0001 M',
            hit_rate: null,
            hit_rate_text: '—',
            requests: 1,
            user_turns: 0,
            quality: 'authoritative',
          },
        ],
        workspaces: [],
        sessions: [],
      },
      by_source: {
        kimi: {
          models: [
            {
              id: 'k2',
              label: 'k2',
              miss: 40,
              cache_read: 0,
              cache_create: 0,
              output: 0,
              total: 40,
              miss_m: '0.0000 M',
              cache_read_m: '0.00 M',
              cache_create_m: '0.00 M',
              output_m: '0.00 M',
              total_m: '0.0000 M',
              hit_rate: null,
              hit_rate_text: '—',
              requests: 1,
              user_turns: 0,
              quality: 'authoritative',
            },
          ],
          workspaces: [],
          sessions: [],
        },
      },
      by_vendor: {},
    }
    expect(selectDrill(payload, { kind: 'all', id: 'all' }).models[0].total).toBe(100)
    expect(selectDrill(payload, { kind: 'source', id: 'kimi' }).models[0].total).toBe(40)
  })
})
