import { describe, expect, it } from 'vitest'
import { rankHint } from './community'

describe('community display', () => {
  it('does not invent a rank from local numbers', () => {
    const hint = rankHint(
      { enabled: true, metric: 'tokens', self_reported: true, note: '', today: { status: 'ok' }, all: { status: 'ok' } },
      { status: 'insufficient_participants' },
    )
    expect(hint).toContain('参与者还不够')
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
