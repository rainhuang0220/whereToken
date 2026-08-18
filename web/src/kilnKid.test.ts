import { describe, expect, it } from 'vitest'
import { kimiLogo, kilnKidFrame, kilnKidMood, moonGlyph, moonGlyphs } from './kilnKid'

describe('kilnKid', () => {
  it('copies the Kimi welcome mark', () => {
    expect(kimiLogo).toEqual(['▐█▛█▛█▌', '▐█████▌'])
    expect(kilnKidFrame(0)).toBe('▐█▛█▛█▌\n▐█████▌')
  })

  it('copies the Kimi moon spinner', () => {
    expect(moonGlyphs[0]).toBe('🌑')
    expect(moonGlyphs[4]).toBe('🌕')
    expect(moonGlyph(0)).toBe(moonGlyph(moonGlyphs.length))
  })

  it('names fidgets as gerunds', () => {
    expect(kilnKidMood(0)).toBe('挠头中')
    expect(kilnKidMood(2)).toBe('搬煤中')
  })
})
