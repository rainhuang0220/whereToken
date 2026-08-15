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

function clearMotionStyles(els: HTMLElement[]) {
  markWillChange(els, 'auto')
  for (const el of els) el.classList?.remove?.('is-flipping')
}

function toEls(targets: gsap.TweenTarget | undefined | null): HTMLElement[] {
  if (!targets) return []
  if (typeof targets === 'string') {
    if (typeof document === 'undefined') return []
    return [...document.querySelectorAll<HTMLElement>(targets)]
  }
  if (Array.isArray(targets)) {
    return targets.filter((el): el is HTMLElement => Boolean(el) && typeof el === 'object')
  }
  if (typeof NodeList !== 'undefined' && targets instanceof NodeList) {
    return [...targets] as HTMLElement[]
  }
  return [targets as HTMLElement]
}

function contextScope(el: HTMLElement): Element | undefined {
  return typeof Element !== 'undefined' && el instanceof Element ? el : undefined
}

function hideNow(el: HTMLElement) {
  el.style.opacity = '0'
  el.style.visibility = 'hidden'
  el.style.pointerEvents = 'none'
}

function showNow(el: HTMLElement) {
  el.style.opacity = ''
  el.style.visibility = ''
  el.style.pointerEvents = ''
  el.style.transform = ''
  el.style.willChange = 'auto'
  el.classList?.remove?.('is-flipping')
}

export function settleGrid(hero: HTMLElement | null | undefined, others: HTMLElement[]) {
  const live = typeof document !== 'undefined'
  if (hero) {
    hero.classList?.remove?.('hero')
    hero.classList?.remove?.('is-flipping')
    if (live) gsap.set(hero, { clearProps: 'transform' })
    else hero.style.transform = ''
    hero.style.willChange = 'auto'
  }
  for (const el of others) {
    el.classList?.remove?.('gone')
    el.classList?.remove?.('is-flipping')
    if (live) gsap.set(el, { clearProps: 'opacity,visibility,transform' })
    showNow(el)
  }
}

export type FlipState = ReturnType<typeof Flip.getState>

export type MotionHandle = {
  revert: () => void
  kill: () => void
  reverse: () => Promise<void>
  play: () => void
  isActive: () => boolean
  canReverse: () => boolean
}

const idleHandle = (): MotionHandle => ({
  revert() {},
  kill() {},
  async reverse() {},
  play() {},
  isActive: () => false,
  canReverse: () => false,
})

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

export function fadeInnerPreview(
  el: HTMLElement | null | undefined,
  opts: { reduced: boolean },
): Promise<void> {
  if (!el) return Promise.resolve()
  if (opts.reduced) {
    el.style.opacity = '0'
    return Promise.resolve()
  }
  return new Promise((resolve) => {
    let settled = false
    const done = () => {
      if (settled) return
      settled = true
      resolve()
    }
    gsap.to(el, {
      opacity: 0,
      duration: 0.1,
      ease: 'none',
      overwrite: true,
      onComplete: done,
      onInterrupt: done,
    })
  })
}

export function afterPaint(): Promise<void> {
  return new Promise((resolve) => {
    if (typeof requestAnimationFrame !== 'function') {
      resolve()
      return
    }
    requestAnimationFrame(() => {
      requestAnimationFrame(() => resolve())
    })
  })
}

export function stopGalleryMotion(targets: gsap.TweenTarget): void {
  ensureFlip()
  const els = toEls(targets)
  try {
    Flip.killFlipsOf(els, false)
  } catch {
    /* Flip not registered in node tests without a window plugin path */
  }
  try {
    gsap.killTweensOf(els)
  } catch {
    /* fake elements in node unit tests have no document */
  }
  clearMotionStyles(els)
}

export function captureFlip(
  targets: gsap.TweenTarget,
  mode: 'rest' | 'current' = 'current',
): FlipState | null {
  ensureFlip()
  const els = toEls(targets)
  stopGalleryMotion(els)
  if (mode === 'rest' && els.length) {
    try {
      gsap.set(els, { x: 0, y: 0, scale: 1, rotation: 0 })
    } catch {
      /* fake elements in node unit tests */
    }
  }
  if (typeof Flip?.getState !== 'function') return null
  try {
    return Flip.getState(targets)
  } catch {
    return null
  }
}

const flipVars = (hero: HTMLElement) => ({
  duration: 0.34,
  ease: 'power2.inOut',
  absolute: true,
  scale: true,
  simple: true,
  overwrite: true,
  toggleClass: 'is-flipping',
  targets: hero,
})

function bindTimeline(
  tl: gsap.core.Timeline,
  ctx: gsap.Context,
  involved: HTMLElement[],
): MotionHandle {
  return {
    revert() {
      tl.kill()
      ctx.revert()
      clearMotionStyles(involved)
    },
    kill() {
      tl.eventCallback('onComplete', null)
      tl.eventCallback('onReverseComplete', null)
      tl.eventCallback('onInterrupt', null)
      tl.kill()
    },
    play() {
      tl.eventCallback('onReverseComplete', null)
      tl.play()
    },
    isActive() {
      return tl.isActive()
    },
    canReverse() {
      return tl.progress() > 0
    },
    reverse() {
      return new Promise<void>((resolve) => {
        const done = () => {
          clearMotionStyles(involved)
          resolve()
        }
        if (tl.progress() === 0) {
          done()
          return
        }
        tl.eventCallback('onReverseComplete', done)
        tl.eventCallback('onInterrupt', done)
        tl.reverse()
      })
    },
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
  if (reduced || !state) {
    for (const el of others) hideNow(el)
    onSettled?.()
    return idleHandle()
  }
  const involved = [hero, ...others]
  let tl!: gsap.core.Timeline
  const ctx = gsap.context(() => {
    tl = gsap.timeline({
      defaults: { ease: 'power2.inOut' },
      onStart() {
        markWillChange([hero], 'transform, opacity')
      },
      onComplete() {
        markWillChange([hero], 'auto')
        onSettled?.()
      },
    })
    tl.add(Flip.from(state, flipVars(hero)), 0)
    if (others.length) {
      tl.to(others, { autoAlpha: 0, duration: 0.22, ease: 'power2.out', overwrite: true }, 0)
    }
  }, contextScope(hero))
  return bindTimeline(tl, ctx, involved)
}

export function restoreGallery(opts: {
  hero: HTMLElement
  others: HTMLElement[]
  reduced: boolean
  state: FlipState | null
  onSettled?: () => void
}): MotionHandle {
  ensureFlip()
  const { hero, others, reduced, state, onSettled } = opts
  if (reduced || !state) {
    for (const el of others) showNow(el)
    showNow(hero)
    onSettled?.()
    return idleHandle()
  }
  const involved = [hero, ...others]
  let tl!: gsap.core.Timeline
  const ctx = gsap.context(() => {
    tl = gsap.timeline({
      defaults: { ease: 'power2.inOut' },
      onStart() {
        markWillChange([hero], 'transform, opacity')
      },
      onComplete() {
        markWillChange([hero], 'auto')
        gsap.set(hero, { clearProps: 'transform' })
        gsap.set(others, { clearProps: 'opacity,visibility,transform' })
        clearMotionStyles(involved)
        onSettled?.()
      },
    })
    if (others.length) {
      for (const el of others) hideNow(el)
    }
    tl.add(
      Flip.from(state, {
        ...flipVars(hero),
        duration: 0.32,
      }),
      0,
    )
    if (others.length) {
      tl.to(others, { autoAlpha: 1, duration: 0.18, ease: 'power2.out', overwrite: true })
    }
  }, contextScope(hero))
  return bindTimeline(tl, ctx, involved)
}
