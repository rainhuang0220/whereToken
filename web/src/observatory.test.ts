import { describe, expect, it } from 'vitest'
import {
  collectKilnMouth,
  observatoryCursorWindowHint,
  observatoryDegradedLines,
  observatoryEmptyHint,
  observatoryHasDrill,
  observatoryHasSlice,
  observatoryInsightCaption,
  observatoryScanErrorHint,
} from './observatory'
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

describe('observatoryCursorWindowHint', () => {
  it('tells the truth when Cursor tokens came from the 53-week account API', () => {
    expect(
      observatoryCursorWindowHint({
        by_source: [row({ id: 'cursor', label: 'Cursor', quality: 'authoritative', total: 1_680_000_000 })],
      }),
    ).toBe('Cursor 的 token 列是近 53 周账号用量；请求和回合仍是本机全部会话。')
  })

  it('stays quiet when Cursor is only local bubbles', () => {
    expect(
      observatoryCursorWindowHint({
        by_source: [row({ id: 'cursor', label: 'Cursor', quality: 'degraded', total: 0 })],
      }),
    ).toBe('')
  })
})

describe('observatoryScanErrorHint', () => {
  it('explains a 409 when there is no previous scan to keep on screen', () => {
    expect(observatoryScanErrorHint('煅烧进行中', false)).toBe('另一处正在刷新。等这次煅烧结束再点。')
    expect(observatoryScanErrorHint('scan 409', false)).toBe('另一处正在刷新。等这次煅烧结束再点。')
  })

  it('still explains a 409 when a previous payload is on screen', () => {
    expect(observatoryScanErrorHint('煅烧进行中', true)).toBe('另一处正在刷新。等这次煅烧结束再点。')
  })
})

describe('observatoryHasSlice', () => {
  it('hides ranking tables when the first scan found nothing', () => {
    expect(observatoryHasSlice({ by_source: [], by_vendor: [] })).toBe(false)
    expect(observatoryHasSlice({ by_source: [row({ id: 'kimi', label: 'Kimi Code' })], by_vendor: [] })).toBe(true)
  })
})

describe('observatoryHasDrill', () => {
  it('hides empty drill ledgers', () => {
    expect(observatoryHasDrill({ models: [], workspaces: [], sessions: [] })).toBe(false)
    expect(observatoryHasDrill({ models: [row({ id: 'k3', label: 'k3' })], workspaces: [], sessions: [] })).toBe(true)
  })
})

describe('collectKilnMouth', () => {
  it('keeps footnotes out of the hero and in one list', () => {
    expect(
      collectKilnMouth({
        offlineBanner: 'offline · 只用本机账本，没有请求 Cursor/Trae 云端',
        claudeDegraded: true,
        loginHint: 'Trae 需要 IDE 已登录。登录后点「刷新」重新煅烧本机账本；浏览器重载只会显示上次结果。',
      }),
    ).toEqual([
      'offline · 只用本机账本，没有请求 Cursor/Trae 云端',
      'Claude Code 日志为降级质量：同一请求取最大值，输入/输出可能偏低。',
      'Trae 需要 IDE 已登录。登录后点「刷新」重新煅烧本机账本；浏览器重载只会显示上次结果。',
    ])
  })

  it('drops empty slots', () => {
    expect(collectKilnMouth({})).toEqual([])
  })
})

describe('observatoryEmptyHint', () => {
  const zeroAll = row({ id: 'all', label: '合计', quality: 'authoritative' })

  it('explains a first visit with no ledgers the same way the CLI does', () => {
    expect(
      observatoryEmptyHint({
        all: zeroAll,
        by_source: [],
        by_vendor: [],
      }),
    ).toBe(
      '本机没有找到账本。装好任一受支持的工具并跑一次就会有数；Cursor / Trae 需要已登录。wheretoken sources 列出全部受支持的工具。',
    )
  })

  it('stays quiet when any token or request exists', () => {
    expect(
      observatoryEmptyHint({
        all: { ...zeroAll, total: 1185, requests: 2 },
        by_source: [],
        by_vendor: [],
      }),
    ).toBe('')
  })

  it('stays quiet when a tool row was discovered even if usage is still zero', () => {
    expect(
      observatoryEmptyHint({
        all: zeroAll,
        by_source: [row({ id: 'cursor', label: 'Cursor', quality: 'degraded' })],
        by_vendor: [],
      }),
    ).toBe('')
  })
})

describe('observatoryInsightCaption', () => {
  it('stays quiet on the all axis and warns when the kiln wall is filtered', () => {
    expect(observatoryInsightCaption({ kind: 'all' })).toBe('')
    expect(observatoryInsightCaption({ kind: 'source' })).toBe('相对窗口合计，不是当前窑墙轴。')
    expect(observatoryInsightCaption({ kind: 'vendor' })).toBe('相对窗口合计，不是当前窑墙轴。')
  })
})
