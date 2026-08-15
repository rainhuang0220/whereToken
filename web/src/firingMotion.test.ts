import { afterEach, describe, expect, it, vi } from 'vitest'
import gsap from 'gsap'
import { tweenCharge } from './firingMotion'

describe('firing motion', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('animates scaleX only, and skips duration when reduced motion is on', () => {
    const to = vi.spyOn(gsap, 'to')
    const el = { style: { willChange: '', transform: '' } } as unknown as HTMLElement
    tweenCharge(el, 0.5, false)
    tweenCharge(el, 1, true)
    expect(to).toHaveBeenCalled()
    const live = to.mock.calls[0]?.[1] as { scaleX?: number; duration?: number; autoAlpha?: number }
    const reduced = to.mock.calls[1]?.[1] as { scaleX?: number; duration?: number }
    expect(live.scaleX).toBe(0.5)
    expect(live.duration).toBeGreaterThan(0)
    expect(live).not.toHaveProperty('width')
    expect(live).not.toHaveProperty('height')
    expect(reduced.scaleX).toBe(1)
    expect(reduced.duration).toBe(0)
  })
})
