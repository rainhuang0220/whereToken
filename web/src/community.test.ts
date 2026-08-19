import { describe, expect, it } from 'vitest'
import { costCaption, rankCaption, rankHint } from './community'

describe('community display', () => {
  it('never prints $0 for unavailable cost', () => {
    expect(costCaption({ cost_status: 'unavailable', total: 100 })).toBe('—')
    expect(costCaption({ cost_usd: '$12.0000', cost_status: 'complete', total: 100 })).toBe('$12.0000')
  })

  it('never prints #0 for missing rank', () => {
    expect(rankCaption(undefined)).toBe('—')
    expect(rankCaption({ status: 'unavailable' })).toBe('—')
    expect(rankCaption({ status: 'network_error' })).toBe('—')
    expect(rankCaption({ status: 'network_error', rank: 0 })).toBe('—')
    expect(rankCaption({ status: 'ok', display: '#0 / 20', rank: 0 })).toBe('—')
    expect(rankCaption({ status: 'ok', display: '#37 / 842', rank: 37, participants: 842 })).toBe(
      '#37 / 842',
    )
  })

  it('does not invent a rank from local numbers', () => {
    const hint = rankHint(
      { enabled: true, metric: 'tokens', self_reported: true, note: '', today: { status: 'ok' }, all: { status: 'ok' } },
      { status: 'insufficient_participants' },
    )
    expect(hint).toContain('参与者还不够')
    expect(rankCaption({ status: 'insufficient_participants', participants: 3 })).toBe('—')
  })

  it('does not imply a live board when rank is unavailable', () => {
    expect(rankHint(undefined, { status: 'unavailable' })).toBe('社区排名暂不可用')
    expect(rankHint()).toBe('社区排名暂不可用')
    expect(rankHint(undefined, { status: 'unavailable' })).not.toContain('全球')
  })

  it('does not describe Community Rank as global or worldwide', () => {
    const hint = rankHint(
      { enabled: true, metric: 'tokens', self_reported: true, note: 'ignored', today: { status: 'ok' }, all: { status: 'ok' } },
      { status: 'ok', rank: 37, display: '#37 / 842', participants: 842 },
    )
    expect(hint).toContain('不是全球')
    expect(hint).toContain('全世界')
    expect(hint).toContain('全体 AI 用户')
    expect(hint).toContain('不是经过审计的竞技排行榜')
  })
})
