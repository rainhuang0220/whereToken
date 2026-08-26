import { describe, expect, it } from 'vitest'
import { kilnKidMood } from './kilnKid'

describe('kilnKid', () => {
  it('names fidgets as gerunds', () => {
    expect(kilnKidMood(0)).toBe('挠头中')
    expect(kilnKidMood(2)).toBe('搬煤中')
  })
})
