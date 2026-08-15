import { describe, expect, it } from 'vitest'
import { observatoryDegradedLines } from './observatory'
import type { SliceView, SummaryPayload } from './types'

function row(partial: Partial<SliceView> & Pick<SliceView, 'id' | 'label'>): SliceView {
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
    quality: 'degraded',
    ...partial,
  }
}

const emptyPayload: Pick<SummaryPayload, 'by_source' | 'by_vendor'> = {
  by_source: [],
  by_vendor: [],
}

describe('observatoryDegradedLines', () => {
  it('shows Trae source error on the home observatory even when 按厂家 has no Trae row', () => {
    const lines = observatoryDegradedLines({
      ...emptyPayload,
      by_source: [
        row({
          id: 'trae',
          label: 'Trae',
          error: '登录态在加密存储中，没有可读的 JWT 文件',
        }),
      ],
    })
    expect(lines).toEqual(['Trae：登录态在加密存储中，没有可读的 JWT 文件'])
  })

  it('also shows a vendor row error when that vendor has a degraded reason', () => {
    const lines = observatoryDegradedLines({
      by_source: [
        row({
          id: 'trae',
          label: 'Trae',
          error: '登录态在加密存储中，没有可读的 JWT 文件',
        }),
      ],
      by_vendor: [
        row({
          id: 'deepseek',
          label: 'DeepSeek',
          error: '登录态在加密存储中，没有可读的 JWT 文件',
        }),
      ],
    })
    expect(lines).toEqual([
      'Trae：登录态在加密存储中，没有可读的 JWT 文件',
      'DeepSeek：登录态在加密存储中，没有可读的 JWT 文件',
    ])
  })

  it('does not invent a line from 降级 without an error string', () => {
    expect(
      observatoryDegradedLines({
        ...emptyPayload,
        by_source: [row({ id: 'trae', label: 'Trae' })],
      }),
    ).toEqual([])
  })
})
