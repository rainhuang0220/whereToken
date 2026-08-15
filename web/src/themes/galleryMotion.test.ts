import { describe, expect, it } from 'vitest'
import { expandGallery, prefersReducedMotion, restoreGallery } from './galleryMotion'

function fakeEl(): { style: Record<string, string> } {
  return { style: {} }
}

describe('gallery motion', () => {
  it('treats prefers-reduced-motion as skip Flip', () => {
    expect(prefersReducedMotion(() => ({ matches: true }) as MediaQueryList)).toBe(true)
    expect(prefersReducedMotion(() => ({ matches: false }) as MediaQueryList)).toBe(false)
  })

  it('reduced expand hides others instantly and does not throw', () => {
    const hero = fakeEl() as unknown as HTMLElement
    const others = [fakeEl(), fakeEl()] as unknown as HTMLElement[]
    expect(() => {
      expandGallery({
        hero,
        others,
        reduced: true,
        state: null,
      })
    }).not.toThrow()
    expect(() => {
      restoreGallery({
        hero,
        others,
        reduced: true,
        state: null,
      })
    }).not.toThrow()
  })
})
