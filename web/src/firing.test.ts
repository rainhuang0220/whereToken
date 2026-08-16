import { describe, expect, it } from 'vitest'
import { chargeAmount, parseSSEBlock, scanEventError, type ScanProgress } from './firing'

describe('firing charge', () => {
  it('maps reading index 1 of 6 to an empty charge, done 6 of 6 to full', () => {
    const reading: ScanProgress = {
      source: 'claude',
      label: '正在读 Claude Code…',
      index: 1,
      total: 6,
      status: 'reading',
    }
    expect(chargeAmount(reading)).toBe(0)
    expect(chargeAmount({ ...reading, index: 3, status: 'done' })).toBe(3 / 6)
    expect(chargeAmount({ ...reading, index: 6, status: 'done' })).toBe(1)
  })

  it('stays at 0 when progress is missing', () => {
    expect(chargeAmount(null)).toBe(0)
  })
})

describe('scan SSE', () => {
  it('parses progress then complete without requiring a trailing blank line after the last event', () => {
    const raw = [
      'event: progress',
      'data: {"source":"trae","label":"正在读 Trae…","index":6,"total":6,"status":"reading"}',
      '',
      'event: complete',
      'data: {"all":{"total_m":"1.07 M"},"scanned_at":"2026-08-16T01:00:00+08:00"}',
      '',
    ].join('\n')
    const events = parseSSEBlock(raw)
    expect(events.map((e) => e.event)).toEqual(['progress', 'complete'])
    expect(events[0].data).toMatch(/"source":"trae"/)
    expect(events[1].data).toMatch(/scanned_at/)
    expect(events[1].data).not.toMatch(/eyJ/)
  })

  it('surfaces the server error event instead of hanging as scan incomplete', () => {
    const events = parseSSEBlock('event: error\ndata: {"error":"encode"}\n\n')
    expect(scanEventError(events[0])).toBe('encode')
    expect(scanEventError({ event: 'complete', data: '{}' })).toBe('')
  })
})
