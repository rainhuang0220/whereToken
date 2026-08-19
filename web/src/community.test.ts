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
})
