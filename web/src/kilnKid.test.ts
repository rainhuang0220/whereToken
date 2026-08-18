import { describe, expect, it } from 'vitest'
import { kilnKidFrame, kilnKidFrames, kilnKidMood, kilnKidPose, kilnKidPoses } from './kilnKid'

describe('kilnKid', () => {
  it('is a three-line face, not a spark bar', () => {
    for (const pose of kilnKidPoses) {
      expect(kilnKidFrames[pose]).toHaveLength(3)
      expect(kilnKidFrames[pose].join('')).not.toMatch(/[▁▂▃▄▅▆]/)
    }
    expect(kilnKidFrame(0).split('\n')).toHaveLength(3)
  })

  it('wraps ticks', () => {
    expect(kilnKidPose(0)).toBe(kilnKidPose(kilnKidPoses.length))
    expect(kilnKidFrame(0)).toBe(kilnKidFrame(kilnKidPoses.length))
  })

  it('names each fidget as a gerund', () => {
    expect(kilnKidMood(0)).toBe('挠头中')
    expect(kilnKidMood(2)).toBe('搬煤中')
  })
})
