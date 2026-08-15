import { describe, expect, it } from 'vitest'
import { columnsFrom } from './format'
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
