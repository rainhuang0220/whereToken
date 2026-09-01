import { describe, expect, it } from 'vitest'
import { columnsFrom, costCaption, costHonestyNote, costKPI, derivationCaption, formatCount, hitBand, qualityCaption, tokenCell } from './format'
import type { SliceView } from './types'

describe('columnsFrom', () => {
  it('returns preformatted display strings without dividing by 1e6', () => {
    const view: SliceView = {
      id: 'kimi',
      label: 'Kimi Code',
      miss: 150,
      cache_read: 1000,
      cache_create: 20,
      output: 15,
      total: 1185,
      miss_m: '0.0002 M',
      cache_read_m: '0.0010 M',
      cache_create_m: '0.0000 M',
      output_m: '0.0000 M',
      total_m: '0.0012 M',
      hit_rate: 85.1,
      hit_rate_text: '85.1%',
      requests: 2,
      user_turns: 1,
      quality: 'authoritative',
    }
    expect(columnsFrom(view)).toEqual([
      '0.0002 M',
      '0.0010 M',
      '0.0000 M',
      '0.0000 M',
      '0.0012 M',
      '85.1%',
    ])
  })
})

describe('hitBand', () => {
  it('matches the CLI green / lemon / red bands', () => {
    expect(hitBand('—')).toBe('none')
    expect(hitBand('89.9%')).toBe('hi')
    expect(hitBand('70.0%')).toBe('hi')
    expect(hitBand('50.0%')).toBe('mid')
    expect(hitBand('10.0%')).toBe('lo')
  })
})

describe('qualityCaption', () => {
  it('names the four quality states in product language', () => {
    expect(qualityCaption('authoritative')).toBe('完整')
    expect(qualityCaption('degraded')).toBe('降级')
    expect(qualityCaption('estimated')).toBe('估算')
    expect(qualityCaption('absent')).toBe('数据不可用')
  })
})

describe('derivationCaption', () => {
  it('names how a number was produced', () => {
    expect(derivationCaption('deduplicated')).toBe('按请求去重')
    expect(derivationCaption('raw,derived')).toBe('原始字段 · 推导值')
  })
})

describe('tokenCell', () => {
  it('does not present absent usage as zero', () => {
    expect(tokenCell('0.00 M', 'absent')).toBe('不可用')
    expect(tokenCell('1.20 M', 'authoritative')).toBe('1.20 M')
    expect(tokenCell('0.00 M', 'degraded', { total: 0, requests: 0, user_turns: 0 })).toBe('不可用')
    expect(tokenCell('0.00 M', 'degraded', { total: 0, requests: 12, user_turns: 3 })).toBe('0.00 M')
  })
})

describe('costCaption', () => {
  it('prints backend cost_usd and never invents $0', () => {
    expect(costCaption({ cost_usd: '$12.0000', cost_status: 'complete', total: 10 })).toBe('$12.0000')
    expect(costCaption({ cost_usd: '$1.0000', cost_status: 'partial', total: 10 })).toBe('$1.0000 · 部分')
    expect(costCaption({ cost_status: 'unavailable', total: 10 })).toBe('—')
    expect(costCaption({ cost_status: 'unavailable', total: 0 })).toBe('')
    expect(costCaption({ cost_usd: '$0.0000', cost_status: 'complete', total: 10 })).toBe('')
    expect(costKPI({ cost_usd: '$0.00', cost_status: 'complete', total: 1 })).toBe('—')
  })
})

describe('costKPI', () => {
  it('uses em dash when the estimate is unavailable', () => {
    expect(costKPI({ cost_usd: '$12.0000', cost_status: 'complete', total: 10 })).toBe('$12.0000')
    expect(costKPI({ cost_status: 'unavailable', total: 10 })).toBe('—')
  })
})

describe('costHonestyNote', () => {
  it('footnotes complete 估价 as not a bill', () => {
    expect(costHonestyNote({ cost_usd: '$12.0000', cost_status: 'complete', total: 10 })).toBe(
      '估价 $12.0000 · API 标价等价，不是订阅账单',
    )
    expect(costHonestyNote({ cost_usd: '$1.0000', cost_status: 'partial', total: 10 })).toBe(
      '估价 $1.0000 · 部分无标价 · API 标价等价，不是订阅账单',
    )
    expect(costHonestyNote({ cost_status: 'unavailable', total: 10 })).toBe('估价不可用 · 不会写成 $0')
    expect(costHonestyNote({ cost_usd: '$0.0000', cost_status: 'complete', total: 10 })).toBe('')
  })
})

describe('formatCount', () => {
  it('groups thousands the same way the CLI table does', () => {
    expect(formatCount(0)).toBe('0')
    expect(formatCount(999)).toBe('999')
    expect(formatCount(1000)).toBe('1,000')
    expect(formatCount(52927)).toBe('52,927')
    expect(formatCount(2323430000)).toBe('2,323,430,000')
  })
})
