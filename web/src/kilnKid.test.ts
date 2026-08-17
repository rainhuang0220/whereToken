import { describe, expect, it } from 'vitest'
import {
  kilnKidFrame,
  kilnKidFrames,
  kilnKidMood,
  kilnKidPose,
  kilnKidPoses,
  kilnTipAt,
  kilnTips,
} from './kilnKid'

describe('kilnKid', () => {
  it('returns four lines for every pose', () => {
    for (const pose of kilnKidPoses) {
      expect(kilnKidFrames[pose]).toHaveLength(4)
    }
    for (let i = 0; i < kilnKidPoses.length * 2; i++) {
      expect(kilnKidFrame(i).split('\n')).toHaveLength(4)
    }
  })

  it('wraps ticks and negative ticks', () => {
    expect(kilnKidPose(0)).toBe(kilnKidPose(kilnKidPoses.length))
    expect(kilnKidPose(-1)).toBe(kilnKidPoses[kilnKidPoses.length - 1])
    expect(kilnKidFrame(0)).toBe(kilnKidFrame(kilnKidPoses.length))
  })

  it('names each fidget', () => {
    expect(kilnKidMood(0)).toBe('挠头')
    expect(kilnKidMood(1)).toBe('拨算盘')
    expect(kilnKidMood(2)).toBe('投煤')
    expect(kilnKidMood(3)).toBe('煅烧')
  })

  it('rotates kiln tips', () => {
    expect(kilnTipAt(0)).toBe(kilnTips[0])
    expect(kilnTipAt(kilnTips.length)).toBe(kilnTips[0])
    expect(kilnTipAt(1)).not.toBe(kilnTipAt(0))
  })

  it('keeps a tuft and a pot on every frame', () => {
    for (const pose of kilnKidPoses) {
      const [tuft, , , feet] = kilnKidFrames[pose]
      expect(tuft).toMatch(/∩∩/)
      expect(feet).toMatch(/∪∪/)
    }
  })
})
