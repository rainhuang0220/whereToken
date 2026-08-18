import { describe, expect, it } from 'vitest'
import { columnsFrom, derivationCaption, formatCount, hitBand, qualityCaption, tokenCell } from './format'
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
