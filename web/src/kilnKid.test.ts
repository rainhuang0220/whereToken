import { describe, expect, it } from 'vitest'
import {
  kilnEyePhase,
  kilnEyePhases,
  kilnGlyph,
  kilnGlyphs,
  kilnKidFrame,
  kilnKidMood,
  kilnKidPose,
  kilnKidPoses,
} from './kilnKid'

describe('kilnKid', () => {
  it('is a single 2-cell block mark', () => {
    for (let i = 0; i < kilnGlyphs.length * 2; i++) {
      expect(kilnKidFrame(i)).toBe(kilnGlyph(i))
      expect(kilnGlyph(i).length).toBeGreaterThan(0)
    }
  })

  it('wraps ticks', () => {
    expect(kilnKidPose(0)).toBe(kilnKidPose(kilnKidPoses.length))
    expect(kilnKidPose(-1)).toBe(kilnKidPoses[kilnKidPoses.length - 1])
    expect(kilnGlyph(0)).toBe(kilnGlyph(kilnGlyphs.length))
  })

  it('names each fidget as a gerund', () => {
    expect(kilnKidMood(0)).toBe('挠头中')
    expect(kilnKidMood(1)).toBe('拨珠中')
    expect(kilnKidMood(2)).toBe('搬煤中')
    expect(kilnKidMood(3)).toBe('煅烧中')
  })

  it('keeps eight moon-like eye phases', () => {
    expect(kilnEyePhase(0)).not.toEqual(kilnEyePhase(1))
    expect(kilnEyePhase(0)).toEqual(kilnEyePhase(kilnEyePhases.length))
  })
})
