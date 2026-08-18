import { describe, expect, it } from 'vitest'
import { kilnKidFrame, kilnKidFrames, kilnKidMood } from './kilnKid'

describe('kilnKid', () => {
  it('is a three-line clawd slab with two eye bars', () => {
    expect(kilnKidFrames.grin[1]).toContain('▌')
    expect(kilnKidFrames.grin[1].split('▌')).toHaveLength(3)
    expect(kilnKidFrame(0).split('\n')).toHaveLength(3)
  })

  it('names fidgets as gerunds', () => {
    expect(kilnKidMood(0)).toBe('挠头中')
    expect(kilnKidMood(2)).toBe('搬煤中')
  })
})
