import { existsSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import {
  KEY_CODES,
  MAIN_ROWS,
  applyPress,
  applyRelease,
  createKeyboardSession,
  isGap,
} from './mockKeyboard'

const DIR = dirname(fileURLToPath(import.meta.url))

function rowCodes(i: number): string[] {
  return MAIN_ROWS[i].filter((slot) => !isGap(slot)).map((slot) => slot.code)
}

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
  it('is a 75% notebook board: Esc, A, space, enter, up, backspace; no numpad codes', () => {
    for (const code of ['Escape', 'KeyA', 'Space', 'Enter', 'ArrowUp', 'Backspace']) {
      expect(KEY_CODES.has(code), code).toBe(true)
    }
    expect([...KEY_CODES].filter((code) => code.startsWith('Numpad'))).toEqual([])
  })

  it('packs six compact rows with Mac modifiers and corner arrows, no TKL gaps', () => {
    expect(MAIN_ROWS).toHaveLength(6)
    expect(MAIN_ROWS[0].some(isGap)).toBe(false)

    const fn = rowCodes(0)
    const nums = rowCodes(1)
    const qwerty = rowCodes(2)
    const home = rowCodes(3)
    const shift = rowCodes(4)
    const bottom = rowCodes(5)

    expect(fn[0]).toBe('Escape')
    expect(fn.slice(1, 13)).toEqual([
      'F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10', 'F11', 'F12',
    ])
    expect(fn).toHaveLength(16)
    expect(nums[0]).toBe('Backquote')
    expect(nums.at(-1)).toBe('Backspace')
    expect(qwerty[0]).toBe('Tab')
    expect(qwerty).toContain('KeyP')
    expect(home[0]).toBe('CapsLock')
    expect(home).toContain('KeyA')
    expect(home.at(-1)).toBe('Enter')
    expect(shift[0]).toBe('ShiftLeft')
    expect(shift).toContain('ArrowUp')
    expect(shift.at(-1)).not.toBe('ShiftRight')
    expect(bottom.slice(0, 4)).toEqual(['Fn', 'ControlLeft', 'AltLeft', 'MetaLeft'])
    expect(bottom).toContain('Space')
    expect(bottom.slice(-3)).toEqual(['ArrowLeft', 'ArrowDown', 'ArrowRight'])
  })

  it('renders one deck in the theme mock, not a main | arrows | pad split', () => {
    const vue = readFileSync(join(DIR, 'MockKeyboard.vue'), 'utf8')
    expect(vue).not.toMatch(/kb-pad|PAD_KEYS|NAV_KEYS|kb-nav/)
    const css = readFileSync(join(DIR, '..', 'styles.css'), 'utf8')
    expect(css).not.toMatch(/\.kb-pad\b/)
    expect(css).not.toMatch(/\.kb-nav\b/)
    expect(existsSync(join(DIR, 'mockWall.ts'))).toBe(false)
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
