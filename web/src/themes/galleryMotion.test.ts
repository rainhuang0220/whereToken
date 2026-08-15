import { afterEach, describe, expect, it, vi } from 'vitest'
import gsap from 'gsap'
import { Flip } from 'gsap/Flip'
import {
  afterPaint,
  captureFlip,
  expandGallery,
  fadeInnerPreview,
  prefersReducedMotion,
  restoreGallery,
  settleGrid,
  stopGalleryMotion,
} from './galleryMotion'

function fakeEl(): HTMLElement {
  const names = new Set<string>()
  const el = {
    style: { willChange: '', transform: '', opacity: '', visibility: '', pointerEvents: '' },
    classList: {
      add(name: string) {
        names.add(name)
      },
      remove(...list: string[]) {
        for (const name of list) names.delete(name)
      },
      contains(name: string) {
        return names.has(name)
      },
    },
    querySelector() {
      return null
    },
  }
  return el as unknown as HTMLElement
}

describe('gallery motion', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('treats prefers-reduced-motion as skip Flip', () => {
    expect(prefersReducedMotion(() => ({ matches: true }) as MediaQueryList)).toBe(true)
    expect(prefersReducedMotion(() => ({ matches: false }) as MediaQueryList)).toBe(false)
  })

  it('reduced expand hides others instantly, skips Flip.from, still settles', () => {
    const from = vi.spyOn(Flip, 'from')
    const hero = fakeEl()
    const others = [fakeEl(), fakeEl()]
    let settled = 0
    expect(() => {
      expandGallery({
        hero,
        others,
        reduced: true,
        state: null,
        onSettled() {
          settled += 1
        },
      })
    }).not.toThrow()
    expect(from).not.toHaveBeenCalled()
    expect(settled).toBe(1)
    expect(others[0].style.opacity).toBe('0')
    expect(() => {
      restoreGallery({
        hero,
        others,
        reduced: true,
        state: null,
      })
    }).not.toThrow()
    expect(from).not.toHaveBeenCalled()
    expect(others[0].style.willChange === '' || others[0].style.willChange === 'auto').toBe(true)
  })

  it('Flip.from uses scale+absolute+simple on the hero only, siblings fade opacity', () => {
    const flipTl = gsap.timeline({ paused: true })
    const from = vi.spyOn(Flip, 'from').mockReturnValue(flipTl)
    const hero = fakeEl()
    const others = [fakeEl(), fakeEl()]
    const state = { id: 'captured' } as unknown as ReturnType<typeof Flip.getState>

    const handle = expandGallery({
      hero,
      others,
      reduced: false,
      state,
    })

    expect(from).toHaveBeenCalledTimes(1)
    expect(from.mock.calls[0][0]).toBe(state)
    const vars = from.mock.calls[0][1] as Record<string, unknown>
    expect(vars.scale).toBe(true)
    expect(vars.absolute).toBe(true)
    expect(vars.simple).toBe(true)
    expect(vars.targets).toBe(hero)
    expect(vars.overwrite).toBe(true)
    expect(vars.toggleClass).toBe('is-flipping')
    expect(vars.nested).not.toBe(true)

    const src = [
      ...Object.keys(vars),
      JSON.stringify(vars),
    ].join(' ')
    expect(src).not.toMatch(/width|height|top|left/)

    handle.revert()
    expect(hero.style.willChange === '' || hero.style.willChange === 'auto').toBe(true)
    expect(hero.classList.contains('is-flipping')).toBe(false)
  })

  it('restore Flip also targets the hero with scale and clears will-change on revert', () => {
    const from = vi.spyOn(Flip, 'from').mockReturnValue(gsap.timeline({ paused: true }))
    const hero = fakeEl()
    hero.style.willChange = 'transform, opacity'
    const others = [fakeEl()]
    const state = { id: 'restored' } as unknown as ReturnType<typeof Flip.getState>

    const handle = restoreGallery({
      hero,
      others,
      reduced: false,
      state,
    })

    expect(from).toHaveBeenCalledTimes(1)
    const vars = from.mock.calls[0][1] as Record<string, unknown>
    expect(vars.scale).toBe(true)
    expect(vars.absolute).toBe(true)
    expect(vars.simple).toBe(true)
    expect(vars.targets).toBe(hero)
    expect(vars.overwrite).toBe(true)

    handle.revert()
    expect(hero.style.willChange).toBe('auto')
    expect(hero.classList.contains('is-flipping')).toBe(false)
  })

  it('restore flies the hero first, then fades siblings — not at Flip time 0', () => {
    vi.spyOn(Flip, 'from').mockReturnValue(gsap.timeline({ paused: true }))
    const to = vi.spyOn(gsap.core.Timeline.prototype, 'to')
    const others = [fakeEl()]
    restoreGallery({
      hero: fakeEl(),
      others,
      reduced: false,
      state: { id: 'leave' } as unknown as ReturnType<typeof Flip.getState>,
    }).revert()
    const sibling = to.mock.calls.find((c) => c[0] === others)
    expect(sibling).toBeTruthy()
    expect(sibling![1]).toMatchObject({ autoAlpha: 1 })
    expect(sibling![2]).not.toBe(0)
  })

  it('restoreGallery builds a new Flip.from and never reverses the enter timeline', () => {
    const from = vi.spyOn(Flip, 'from').mockReturnValue(gsap.timeline({ paused: true }))
    const reverse = vi.spyOn(gsap.core.Timeline.prototype, 'reverse')
    const onSettled = vi.fn()
    const tlSpy = vi.spyOn(gsap, 'timeline')
    restoreGallery({
      hero: fakeEl(),
      others: [fakeEl()],
      reduced: false,
      state: { id: 'leave' } as unknown as ReturnType<typeof Flip.getState>,
      onSettled,
    }).revert()
    expect(from).toHaveBeenCalledTimes(1)
    expect(reverse).not.toHaveBeenCalled()
    const vars = tlSpy.mock.calls[0][0] as { onComplete?: () => void }
    expect(typeof vars.onComplete).toBe('function')
    expect(onSettled).not.toHaveBeenCalled()
    vars.onComplete?.()
    expect(onSettled).toHaveBeenCalledTimes(1)
  })

  it('enter onSettled is timeline onComplete, not a callback tween that reverse() would rewind', () => {
    vi.spyOn(Flip, 'from').mockReturnValue(gsap.timeline({ paused: true }))
    const add = vi.spyOn(gsap.core.Timeline.prototype, 'add')
    const call = vi.spyOn(gsap.core.Timeline.prototype, 'call')
    const onSettled = vi.fn()
    expandGallery({
      hero: fakeEl(),
      others: [fakeEl()],
      reduced: false,
      state: { id: 'enter' } as unknown as ReturnType<typeof Flip.getState>,
      onSettled,
    }).revert()
    expect(call).not.toHaveBeenCalled()
    for (const args of add.mock.calls) {
      expect(typeof args[0]).not.toBe('function')
    }
    expect(onSettled).not.toHaveBeenCalled()
  })

  it('fadeInnerPreview tweens opacity only for 80–150ms; reduced is instant', async () => {
    const to = vi.spyOn(gsap, 'to').mockImplementation((_t, vars) => {
      const v = vars as { onComplete?: () => void }
      v.onComplete?.()
      return gsap.timeline({ paused: true }) as unknown as gsap.core.Tween
    })
    const el = fakeEl()
    await fadeInnerPreview(el, { reduced: false })
    expect(to).toHaveBeenCalledTimes(1)
    const vars = to.mock.calls[0][1] as Record<string, unknown>
    expect(vars.opacity).toBe(0)
    expect(vars.duration).toBeGreaterThanOrEqual(0.08)
    expect(vars.duration).toBeLessThanOrEqual(0.15)
    expect(vars).not.toHaveProperty('width')
    expect(vars).not.toHaveProperty('height')
    expect(vars).not.toHaveProperty('autoAlpha')
    to.mockClear()
    const instant = fakeEl()
    instant.style.opacity = '1'
    await fadeInnerPreview(instant, { reduced: true })
    expect(to).not.toHaveBeenCalled()
    expect(instant.style.opacity).toBe('0')
  })

  it('stopGalleryMotion kills Flip without completing and clears will-change', () => {
    const kill = vi.spyOn(Flip, 'killFlipsOf').mockImplementation(() => {})
    const hero = fakeEl()
    hero.style.willChange = 'transform, opacity'
    hero.classList.add('is-flipping')
    const other = fakeEl()
    other.style.willChange = 'transform, opacity'

    stopGalleryMotion([hero, other])

    expect(kill).toHaveBeenCalled()
    expect(kill.mock.calls[0][0]).toEqual([hero, other])
    expect(kill.mock.calls[0][1]).toBe(false)
    expect(hero.style.willChange).toBe('auto')
    expect(other.style.willChange).toBe('auto')
    expect(hero.classList.contains('is-flipping')).toBe(false)
  })

  it('afterPaint waits two animation frames so Flip runs after layout', async () => {
    const frames: string[] = []
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
      frames.push('raf')
      cb(0)
      return frames.length
    })
    await afterPaint()
    expect(frames).toEqual(['raf', 'raf'])
    vi.unstubAllGlobals()
  })

  it('captureFlip stops in-flight flips before measuring', () => {
    const kill = vi.spyOn(Flip, 'killFlipsOf').mockImplementation(() => {})
    const getState = vi.spyOn(Flip, 'getState').mockReturnValue({ id: 'state' } as never)
    const hero = fakeEl()
    const state = captureFlip(hero)
    expect(kill).toHaveBeenCalled()
    expect(kill.mock.calls[0][1]).toBe(false)
    expect(getState).toHaveBeenCalled()
    expect(state).toEqual({ id: 'state' })
  })

  it('kill stops motion without reversing content-show callbacks', () => {
    vi.spyOn(Flip, 'from').mockReturnValue(gsap.timeline({ paused: true }))
    const onSettled = vi.fn()
    const handle = expandGallery({
      hero: fakeEl(),
      others: [fakeEl()],
      reduced: false,
      state: { id: 'x' } as unknown as ReturnType<typeof Flip.getState>,
      onSettled,
    })
    expect(typeof handle.kill).toBe('function')
    handle.kill()
    expect(onSettled).not.toHaveBeenCalled()
    handle.revert()
  })

  it('settleGrid drops hero class and sticky will-change', () => {
    const hero = fakeEl()
    hero.style.willChange = 'transform, opacity'
    hero.classList.add('hero')
    hero.classList.add('is-flipping')
    const other = fakeEl()
    other.classList.add('gone')
    other.style.willChange = 'transform, opacity'
    settleGrid(hero, [other])
    expect(hero.classList.contains('hero')).toBe(false)
    expect(hero.classList.contains('is-flipping')).toBe(false)
    expect(hero.style.willChange).toBe('auto')
    expect(other.classList.contains('gone')).toBe(false)
    expect(other.style.willChange).toBe('auto')
  })
})
