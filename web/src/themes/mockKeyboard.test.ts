import { describe, expect, it } from 'vitest'
import {
  KEY_CODES,
  applyPress,
  applyRelease,
  createKeyboardSession,
} from './mockKeyboard'

function fakeKey() {
  const names = new Set<string>()
  const el = {
    style: { willChange: '' },
    classList: {
      add(name: string) {
        names.add(name)
      },
      remove(name: string) {
        names.delete(name)
      },
      contains(name: string) {
        return names.has(name)
      },
    },
  }
  return { el: el as unknown as HTMLElement, names }
}

describe('gallery mock keyboard', () => {
  it('lays out Mac laptop keys plus a numpad, including Esc, A, space, enter, Numpad0', () => {
    for (const code of ['Escape', 'KeyA', 'Space', 'Enter', 'Numpad0']) {
      expect(KEY_CODES.has(code), code).toBe(true)
    }
  })

  it('presses on keydown and clears on keyup instead of latching', () => {
    const session = createKeyboardSession()
    const first = session.keydown({ code: 'KeyA', repeat: false })
    expect(first.animate).toBe(true)
    expect(session.isPressed('KeyA')).toBe(true)
    session.keyup('KeyA')
    expect(session.isPressed('KeyA')).toBe(false)

    const { el, names } = fakeKey()
    applyPress(el, { reduced: true })
    expect(names.has('is-down')).toBe(true)
    applyRelease(el, { reduced: true })
    expect(names.has('is-down')).toBe(false)
  })

  it('keeps a held key pressed on repeat without retriggering the tween', () => {
    const session = createKeyboardSession()
    expect(session.keydown({ code: 'Space', repeat: false }).animate).toBe(true)
    expect(session.keydown({ code: 'Space', repeat: true }).animate).toBe(false)
    expect(session.isPressed('Space')).toBe(true)
  })

  it('reduced-motion press path does not throw', () => {
    const { el } = fakeKey()
    expect(() => applyPress(el, { reduced: true })).not.toThrow()
    expect(() => applyRelease(el, { reduced: true })).not.toThrow()
  })
})
