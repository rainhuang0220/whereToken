import gsap from 'gsap'
import { Flip } from 'gsap/Flip'

let registered = false

function ensureFlip() {
  if (registered) return
  if (typeof window === 'undefined') return
  gsap.registerPlugin(Flip)
  registered = true
}

function markWillChange(els: HTMLElement[], value: string) {
  for (const el of els) el.style.willChange = value
}

export type FlipState = ReturnType<typeof Flip.getState>

export type MotionHandle = {
  revert: () => void
}

function hideNow(el: HTMLElement) {
  el.style.opacity = '0'
  el.style.visibility = 'hidden'
  el.style.pointerEvents = 'none'
  el.style.transform = 'scale(0.92)'
}

function showNow(el: HTMLElement) {
  el.style.opacity = ''
  el.style.visibility = ''
  el.style.pointerEvents = ''
  el.style.transform = ''
}

export function prefersReducedMotion(
  matchMedia: (query: string) => { matches: boolean } = (query) =>
    typeof window !== 'undefined' ? window.matchMedia(query) : { matches: false },
): boolean {
  try {
    return matchMedia('(prefers-reduced-motion: reduce)').matches
  } catch {
    return false
  }
}

export function captureFlip(targets: gsap.TweenTarget): FlipState | null {
  ensureFlip()
  if (typeof Flip?.getState !== 'function') return null
  try {
    return Flip.getState(targets)
  } catch {
    return null
  }
}

export function expandGallery(opts: {
  hero: HTMLElement
  others: HTMLElement[]
  reduced: boolean
  state: FlipState | null
  onSettled?: () => void
}): MotionHandle {
  ensureFlip()
  const { hero, others, reduced, state, onSettled } = opts
  const intro = typeof hero.querySelector === 'function' ? hero.querySelector('.glaze-expand') : null
  if (reduced || !state) {
    for (const el of others) hideNow(el)
    if (intro && 'style' in intro) {
      const node = intro as HTMLElement
      node.style.opacity = '1'
      node.style.visibility = ''
    }
    onSettled?.()
    return { revert() {} }
  }
  const involved = [hero, ...others]
  const tl = gsap.timeline({
    defaults: { ease: 'power2.inOut' },
    onStart() {
      markWillChange(involved, 'transform, opacity')
    },
    onComplete() {
      markWillChange(involved, 'auto')
      onSettled?.()
    },
  })
  tl.add(
    Flip.from(state, {
      duration: 0.34,
      ease: 'power2.inOut',
      absolute: true,
      scale: true,
    }),
    0,
  )
  tl.to(others, { autoAlpha: 0, scale: 0.92, duration: 0.22, stagger: 0.025, ease: 'power2.in' }, 0)
  if (intro) {
    tl.fromTo(intro, { autoAlpha: 0 }, { autoAlpha: 1, duration: 0.18, ease: 'power1.out' }, 0.16)
  }
  return {
    revert() {
      tl.kill()
      markWillChange(involved, 'auto')
    },
  }
}

export function restoreGallery(opts: {
  hero: HTMLElement
  others: HTMLElement[]
  reduced: boolean
  state: FlipState | null
}): MotionHandle {
  ensureFlip()
  const { hero, others, reduced, state } = opts
  if (reduced || !state) {
    for (const el of others) showNow(el)
    showNow(hero)
    return { revert() {} }
  }
  const involved = [hero, ...others]
  const tl = gsap.timeline({
    defaults: { ease: 'power2.inOut' },
    onStart() {
      markWillChange(involved, 'transform, opacity')
    },
    onComplete() {
      markWillChange(involved, 'auto')
      gsap.set(hero, { clearProps: 'transform' })
      gsap.set(others, { clearProps: 'opacity,visibility,transform' })
    },
  })
  tl.add(
    Flip.from(state, {
      duration: 0.32,
      ease: 'power2.inOut',
      absolute: true,
      scale: true,
    }),
    0,
  )
  tl.to(others, { autoAlpha: 1, scale: 1, duration: 0.24, stagger: 0.03, ease: 'power2.out' }, 0.04)
  return {
    revert() {
      tl.kill()
      markWillChange(involved, 'auto')
    },
  }
}
