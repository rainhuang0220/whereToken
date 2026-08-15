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

  it('gives every layout key a rest zone and glaze token, never void or reserved ember-4', () => {
    const forbidden = new Set(['void', 'clay', 'bone', 'ember-4', 'transparent'])
    const zones = new Set(['alpha', 'num', 'fn', 'mod', 'arrow', 'space'])
    const seen = new Set<string>()

    for (const row of MAIN_ROWS) {
      for (const slot of row) {
        if (isGap(slot)) continue
        const key = slot as { code: string; zone?: string; rest?: string; pressed?: string }
        expect(zones.has(key.zone ?? ''), `${key.code} zone`).toBe(true)
        expect(key.rest, `${key.code} rest`).toMatch(/^ember-[123]$/)
        expect(forbidden.has(key.rest ?? ''), `${key.code} rest ${key.rest}`).toBe(false)
        seen.add(key.code)
      }
    }

    expect(seen.size).toBe(KEY_CODES.size)
  })

  it('paints letter, number, F, modifier, arrow, and space as shade bands of one glaze', () => {
    const restOf = (code: string) => {
      for (const row of MAIN_ROWS) {
        for (const slot of row) {
          if (!isGap(slot) && slot.code === code) {
            return slot as { zone: string; rest: string; pressed: string }
          }
        }
      }
      throw new Error(`missing ${code}`)
    }

    expect(restOf('KeyA')).toMatchObject({ zone: 'alpha', rest: 'ember-1', pressed: 'ember-2' })
    expect(restOf('KeyP')).toMatchObject({ zone: 'alpha', rest: 'ember-1', pressed: 'ember-2' })
    expect(restOf('Comma')).toMatchObject({ zone: 'alpha', rest: 'ember-1', pressed: 'ember-2' })
    expect(restOf('Quote')).toMatchObject({ zone: 'alpha', rest: 'ember-1', pressed: 'ember-2' })
    expect(restOf('Backquote')).toMatchObject({ zone: 'num', rest: 'ember-2', pressed: 'ember-3' })
    expect(restOf('Digit1')).toMatchObject({ zone: 'num', rest: 'ember-2', pressed: 'ember-3' })
    expect(restOf('Equal')).toMatchObject({ zone: 'num', rest: 'ember-2', pressed: 'ember-3' })
    expect(restOf('F1')).toMatchObject({ zone: 'fn', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('F12')).toMatchObject({ zone: 'fn', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('Escape')).toMatchObject({ zone: 'mod', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('Tab')).toMatchObject({ zone: 'mod', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('CapsLock')).toMatchObject({ zone: 'mod', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('ShiftLeft')).toMatchObject({ zone: 'mod', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('Backspace')).toMatchObject({ zone: 'mod', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('Enter')).toMatchObject({ zone: 'mod', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('Fn')).toMatchObject({ zone: 'mod', rest: 'ember-3', pressed: 'ember-4' })
    expect(restOf('ArrowUp')).toMatchObject({ zone: 'arrow', rest: 'ember-2', pressed: 'ember-3' })
    expect(restOf('ArrowLeft')).toMatchObject({ zone: 'arrow', rest: 'ember-2', pressed: 'ember-3' })
    expect(restOf('Space')).toMatchObject({ zone: 'space', rest: 'ember-3', pressed: 'ember-4' })
  })

  it('steps press one shade darker in the ember ramp, never black or void', () => {
    const next: Record<string, string> = {
      'ember-1': 'ember-2',
      'ember-2': 'ember-3',
      'ember-3': 'ember-4',
    }

    for (const row of MAIN_ROWS) {
      for (const slot of row) {
        if (isGap(slot)) continue
        const key = slot as { code: string; rest?: string; pressed?: string }
        expect(key.rest, `${key.code} rest`).toMatch(/^ember-[123]$/)
        expect(key.pressed, key.code).toBe(next[key.rest ?? ''])
        expect(key.pressed).not.toBe('void')
        expect(key.pressed).not.toBe('bone')
        expect(String(key.pressed)).not.toMatch(/#000/)
      }
    }

    const css = readFileSync(join(DIR, '..', 'styles.css'), 'utf8')
    const down = css.match(/\.kb-key\.is-down\s*\{([^}]+)\}/)
    expect(down, 'missing .kb-key.is-down').toBeTruthy()
    expect(down![1]).toMatch(/background:\s*var\(--key-pressed\)/)
    expect(down![1]).not.toMatch(/#000|#000000|var\(--void\)|var\(--bone\)/)
    expect(css).not.toMatch(/\.kb-key\[data-fill='void'\]/)
    expect(css).toMatch(/\[data-zone='alpha'\][^{]*\{[^}]*--key-rest:\s*var\(--ember-1\)/)
    expect(css).toMatch(/\[data-zone='num'\][^{]*\{[^}]*--key-rest:\s*var\(--ember-2\)/)
    expect(css).toMatch(/\[data-zone='fn'\][^{]*\{[^}]*--key-rest:\s*var\(--ember-3\)/)
    expect(css).toMatch(/\[data-zone='mod'\][^{]*\{[^}]*--key-rest:\s*var\(--ember-3\)/)
    expect(css).toMatch(/\[data-zone='arrow'\][^{]*\{[^}]*--key-rest:\s*var\(--ember-2\)/)
    expect(css).toMatch(/\[data-zone='space'\][^{]*\{[^}]*--key-rest:\s*var\(--ember-3\)/)

    const vue = readFileSync(join(DIR, 'MockKeyboard.vue'), 'utf8')
    expect(vue).toMatch(/:data-zone="slot\.zone"/)
    expect(vue).not.toMatch(/data-fill/)
  })

  it('rounds keycaps like inner bricks, slightly more, never stadium pills', () => {
    const css = readFileSync(join(DIR, '..', 'styles.css'), 'utf8')
    function rule(selector: string): string {
      const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      const match = css.match(new RegExp(`${escaped}\\s*\\{([^}]+)\\}`))
      expect(match, `missing ${selector} rule`).toBeTruthy()
      return match![1]
    }
    const key = rule('.kb-key')
    expect(key).toMatch(/border-radius:\s*var\(--key-radius/)
    expect(key).not.toMatch(/999px/)
    expect(key).not.toMatch(/border-radius:\s*0\b/)
    const deck = rule('.kb')
    expect(deck).toMatch(/border-radius:\s*var\(--wall-radius/)
  })
})
