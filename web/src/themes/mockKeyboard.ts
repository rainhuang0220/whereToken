import gsap from 'gsap'

export type KeyFill = 'clay' | 'void' | 'bone' | 'ember-1' | 'ember-2' | 'ember-3' | 'ember-4'

export type KeySpec = {
  code: string
  label: string
  u: number
  fill: KeyFill
}

export type Gap = { gap: number }

export type Slot = KeySpec | Gap

export type GridKey = KeySpec & {
  c: number
  r: number
  cs?: number
  rs?: number
}

const EMBERS: KeyFill[] = ['ember-1', 'ember-2', 'ember-3', 'ember-4']

const ALPHA_FILL: Record<string, KeyFill> = {
  Backquote: 'ember-1',
  Digit1: 'ember-2',
  Digit2: 'ember-1',
  Digit3: 'ember-2',
  Digit4: 'ember-3',
  Digit5: 'ember-2',
  Digit6: 'ember-1',
  Digit7: 'ember-2',
  Digit8: 'ember-3',
  Digit9: 'ember-1',
  Digit0: 'ember-2',
  Minus: 'ember-1',
  Equal: 'ember-2',
  KeyQ: 'ember-2',
  KeyW: 'ember-1',
  KeyE: 'ember-3',
  KeyR: 'ember-2',
  KeyT: 'ember-1',
  KeyY: 'ember-2',
  KeyU: 'ember-3',
  KeyI: 'ember-2',
  KeyO: 'ember-3',
  KeyP: 'ember-1',
  BracketLeft: 'ember-2',
  BracketRight: 'ember-1',
  Backslash: 'ember-2',
  KeyA: 'ember-3',
  KeyS: 'ember-3',
  KeyD: 'ember-4',
  KeyF: 'ember-3',
  KeyG: 'ember-2',
  KeyH: 'ember-2',
  KeyJ: 'ember-4',
  KeyK: 'ember-3',
  KeyL: 'ember-3',
  Semicolon: 'ember-2',
  Quote: 'ember-1',
  KeyZ: 'ember-1',
  KeyX: 'ember-2',
  KeyC: 'ember-2',
  KeyV: 'ember-3',
  KeyB: 'ember-1',
  KeyN: 'ember-2',
  KeyM: 'ember-3',
  Comma: 'ember-1',
  Period: 'ember-2',
  Slash: 'ember-1',
}

function ember(code: string, salt: number): KeyFill {
  let n = salt
  for (let i = 0; i < code.length; i++) n = (n * 33 + code.charCodeAt(i)) >>> 0
  return EMBERS[n % 4]
}

function alpha(code: string, label: string, u = 1): KeySpec {
  return { code, label, u, fill: ALPHA_FILL[code] ?? ember(code, 3) }
}

function mod(code: string, label: string, u: number): KeySpec {
  return { code, label, u, fill: 'clay' }
}

function padDigit(code: string, label: string, c: number, r: number, extra: Partial<GridKey> = {}): GridKey {
  return { code, label, u: 1, fill: ember(code, 17), c, r, ...extra }
}

function padMod(code: string, label: string, c: number, r: number, extra: Partial<GridKey> = {}): GridKey {
  return { code, label, u: 1, fill: 'clay', c, r, ...extra }
}

export function isGap(slot: Slot): slot is Gap {
  return 'gap' in slot
}

export const MAIN_ROWS: Slot[][] = [
  [
    mod('Escape', 'esc', 1.55),
    { gap: 0.55 },
    mod('F1', 'f1', 1),
    mod('F2', 'f2', 1),
    mod('F3', 'f3', 1),
    mod('F4', 'f4', 1),
    { gap: 0.45 },
    mod('F5', 'f5', 1),
    mod('F6', 'f6', 1),
    mod('F7', 'f7', 1),
    mod('F8', 'f8', 1),
    { gap: 0.45 },
    mod('F9', 'f9', 1),
    mod('F10', 'f10', 1),
    mod('F11', 'f11', 1),
    mod('F12', 'f12', 1),
  ],
  [
    alpha('Backquote', '`'),
    alpha('Digit1', '1'),
    alpha('Digit2', '2'),
    alpha('Digit3', '3'),
    alpha('Digit4', '4'),
    alpha('Digit5', '5'),
    alpha('Digit6', '6'),
    alpha('Digit7', '7'),
    alpha('Digit8', '8'),
    alpha('Digit9', '9'),
    alpha('Digit0', '0'),
    alpha('Minus', '-'),
    alpha('Equal', '='),
    mod('Backspace', 'delete', 1.85),
  ],
  [
    mod('Tab', 'tab', 1.55),
    alpha('KeyQ', 'Q'),
    alpha('KeyW', 'W'),
    alpha('KeyE', 'E'),
    alpha('KeyR', 'R'),
    alpha('KeyT', 'T'),
    alpha('KeyY', 'Y'),
    alpha('KeyU', 'U'),
    alpha('KeyI', 'I'),
    alpha('KeyO', 'O'),
    alpha('KeyP', 'P'),
    alpha('BracketLeft', '['),
    alpha('BracketRight', ']'),
    alpha('Backslash', '\\', 1.45),
  ],
  [
    mod('CapsLock', 'caps', 1.8),
    alpha('KeyA', 'A'),
    alpha('KeyS', 'S'),
    alpha('KeyD', 'D'),
    alpha('KeyF', 'F'),
    alpha('KeyG', 'G'),
    alpha('KeyH', 'H'),
    alpha('KeyJ', 'J'),
    alpha('KeyK', 'K'),
    alpha('KeyL', 'L'),
    alpha('Semicolon', ';'),
    alpha('Quote', "'"),
    mod('Enter', 'return', 2.15),
  ],
  [
    mod('ShiftLeft', 'shift', 2.35),
    alpha('KeyZ', 'Z'),
    alpha('KeyX', 'X'),
    alpha('KeyC', 'C'),
    alpha('KeyV', 'V'),
    alpha('KeyB', 'B'),
    alpha('KeyN', 'N'),
    alpha('KeyM', 'M'),
    alpha('Comma', ','),
    alpha('Period', '.'),
    alpha('Slash', '/'),
    mod('ShiftRight', 'shift', 2.65),
  ],
  [
    mod('Fn', 'fn', 1),
    mod('ControlLeft', 'control', 1.15),
    mod('AltLeft', 'option', 1.2),
    mod('MetaLeft', 'command', 1.45),
    mod('Space', ' ', 5.85),
    mod('MetaRight', 'command', 1.45),
    mod('AltRight', 'option', 1.2),
  ],
]

export const NAV_KEYS: GridKey[] = [
  { ...mod('ArrowUp', '↑', 1), c: 2, r: 1 },
  { ...mod('ArrowLeft', '←', 1), c: 1, r: 2 },
  { ...mod('ArrowDown', '↓', 1), c: 2, r: 2 },
  { ...mod('ArrowRight', '→', 1), c: 3, r: 2 },
]

export const PAD_KEYS: GridKey[] = [
  padMod('NumLock', 'clear', 1, 1),
  padMod('NumpadEqual', '=', 2, 1),
  padMod('NumpadDivide', '/', 3, 1),
  padMod('NumpadMultiply', '*', 4, 1),
  padDigit('Numpad7', '7', 1, 2),
  padDigit('Numpad8', '8', 2, 2),
  padDigit('Numpad9', '9', 3, 2),
  padMod('NumpadSubtract', '−', 4, 2),
  padDigit('Numpad4', '4', 1, 3),
  padDigit('Numpad5', '5', 2, 3),
  padDigit('Numpad6', '6', 3, 3),
  padMod('NumpadAdd', '+', 4, 3, { rs: 2 }),
  padDigit('Numpad1', '1', 1, 4),
  padDigit('Numpad2', '2', 2, 4),
  padDigit('Numpad3', '3', 3, 4),
  padDigit('Numpad0', '0', 1, 5, { cs: 2, fill: 'ember-1' }),
  padDigit('NumpadDecimal', '.', 3, 5),
  padMod('NumpadEnter', 'enter', 4, 5),
]

export const KEY_CODES: Set<string> = new Set()

for (const row of MAIN_ROWS) {
  for (const slot of row) {
    if (!isGap(slot)) KEY_CODES.add(slot.code)
  }
}
for (const key of NAV_KEYS) KEY_CODES.add(key.code)
for (const key of PAD_KEYS) KEY_CODES.add(key.code)

export type KeyStroke = {
  code: string
  repeat?: boolean
  isComposing?: boolean
  key?: string
  metaKey?: boolean
  ctrlKey?: boolean
  target?: EventTarget | null
}

export function createKeyboardSession() {
  const pressed = new Set<string>()

  return {
    isPressed(code: string) {
      return pressed.has(code)
    },
    keydown(e: KeyStroke) {
      if (e.isComposing || e.key === 'Process') return { animate: false }
      if (!KEY_CODES.has(e.code)) return { animate: false }
      if (e.repeat || pressed.has(e.code)) {
        pressed.add(e.code)
        return { animate: false }
      }
      pressed.add(e.code)
      return { animate: true }
    },
    keyup(code: string) {
      pressed.delete(code)
    },
    releaseAll() {
      const codes = [...pressed]
      pressed.clear()
      return codes
    },
  }
}

export function shouldPreventDefault(e: KeyStroke): boolean {
  if (e.metaKey || e.ctrlKey) return false
  const el = e.target instanceof Element ? e.target : null
  if (el?.closest('input, textarea, select, button, a, [contenteditable="true"]')) return false
  return e.code === 'Space' || e.code.startsWith('Arrow')
}

export function applyPress(el: HTMLElement, opts: { reduced: boolean }) {
  el.classList.add('is-down')
  if (opts.reduced) return
  el.style.willChange = 'transform'
  gsap.to(el, {
    y: 2.4,
    scaleY: 0.9,
    duration: 0.07,
    ease: 'power2.out',
    overwrite: true,
    transformOrigin: '50% 100%',
  })
}

export function applyRelease(el: HTMLElement, opts: { reduced: boolean }) {
  el.classList.remove('is-down')
  if (opts.reduced) {
    el.style.willChange = 'auto'
    return
  }
  gsap.to(el, {
    y: 0,
    scaleY: 1,
    duration: 0.11,
    ease: 'power2.out',
    overwrite: true,
    transformOrigin: '50% 100%',
    onComplete() {
      el.style.willChange = 'auto'
      gsap.set(el, { clearProps: 'transform' })
    },
  })
}
