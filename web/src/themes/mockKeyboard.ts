import gsap from 'gsap'

export type KeyZone = 'alpha' | 'num' | 'fn' | 'mod' | 'arrow' | 'space'
export type EmberToken = 'ember-1' | 'ember-2' | 'ember-3' | 'ember-4'
export type RestToken = 'ember-1' | 'ember-2' | 'ember-3'

export const ZONE_REST = {
  alpha: 'ember-1',
  num: 'ember-2',
  fn: 'ember-3',
  mod: 'ember-3',
  arrow: 'ember-2',
  space: 'ember-3',
} as const satisfies Record<KeyZone, RestToken>

export const PRESS_FROM_REST = {
  'ember-1': 'ember-2',
  'ember-2': 'ember-3',
  'ember-3': 'ember-4',
} as const satisfies Record<RestToken, EmberToken>

export type KeySpec = {
  code: string
  label: string
  u: number
  zone: KeyZone
  rest: RestToken
  pressed: EmberToken
}

export type Gap = { gap: number }

export type Slot = KeySpec | Gap

function key(code: string, label: string, u: number, zone: KeyZone): KeySpec {
  const rest = ZONE_REST[zone]
  return { code, label, u, zone, rest, pressed: PRESS_FROM_REST[rest] }
}

export function isGap(slot: Slot): slot is Gap {
  return 'gap' in slot
}

export const MAIN_ROWS: Slot[][] = [
  [
    key('Escape', 'esc', 1, 'mod'),
    key('F1', 'f1', 1, 'fn'),
    key('F2', 'f2', 1, 'fn'),
    key('F3', 'f3', 1, 'fn'),
    key('F4', 'f4', 1, 'fn'),
    key('F5', 'f5', 1, 'fn'),
    key('F6', 'f6', 1, 'fn'),
    key('F7', 'f7', 1, 'fn'),
    key('F8', 'f8', 1, 'fn'),
    key('F9', 'f9', 1, 'fn'),
    key('F10', 'f10', 1, 'fn'),
    key('F11', 'f11', 1, 'fn'),
    key('F12', 'f12', 1, 'fn'),
    key('PrintScreen', 'scr', 1, 'mod'),
    key('Delete', 'del', 1, 'mod'),
    key('Power', 'pwr', 1, 'mod'),
  ],
  [
    key('Backquote', '`', 1, 'num'),
    key('Digit1', '1', 1, 'num'),
    key('Digit2', '2', 1, 'num'),
    key('Digit3', '3', 1, 'num'),
    key('Digit4', '4', 1, 'num'),
    key('Digit5', '5', 1, 'num'),
    key('Digit6', '6', 1, 'num'),
    key('Digit7', '7', 1, 'num'),
    key('Digit8', '8', 1, 'num'),
    key('Digit9', '9', 1, 'num'),
    key('Digit0', '0', 1, 'num'),
    key('Minus', '-', 1, 'num'),
    key('Equal', '=', 1, 'num'),
    key('Backspace', 'delete', 3, 'mod'),
  ],
  [
    key('Tab', 'tab', 1.5, 'mod'),
    key('KeyQ', 'Q', 1, 'alpha'),
    key('KeyW', 'W', 1, 'alpha'),
    key('KeyE', 'E', 1, 'alpha'),
    key('KeyR', 'R', 1, 'alpha'),
    key('KeyT', 'T', 1, 'alpha'),
    key('KeyY', 'Y', 1, 'alpha'),
    key('KeyU', 'U', 1, 'alpha'),
    key('KeyI', 'I', 1, 'alpha'),
    key('KeyO', 'O', 1, 'alpha'),
    key('KeyP', 'P', 1, 'alpha'),
    key('BracketLeft', '[', 1, 'alpha'),
    key('BracketRight', ']', 1, 'alpha'),
    key('Backslash', '\\', 1, 'alpha'),
    key('Home', 'home', 1, 'mod'),
  ],
  [
    key('CapsLock', 'caps', 1.75, 'mod'),
    key('KeyA', 'A', 1, 'alpha'),
    key('KeyS', 'S', 1, 'alpha'),
    key('KeyD', 'D', 1, 'alpha'),
    key('KeyF', 'F', 1, 'alpha'),
    key('KeyG', 'G', 1, 'alpha'),
    key('KeyH', 'H', 1, 'alpha'),
    key('KeyJ', 'J', 1, 'alpha'),
    key('KeyK', 'K', 1, 'alpha'),
    key('KeyL', 'L', 1, 'alpha'),
    key('Semicolon', ';', 1, 'alpha'),
    key('Quote', "'", 1, 'alpha'),
    key('Enter', 'return', 3.25, 'mod'),
  ],
  [
    key('ShiftLeft', 'shift', 2.25, 'mod'),
    key('KeyZ', 'Z', 1, 'alpha'),
    key('KeyX', 'X', 1, 'alpha'),
    key('KeyC', 'C', 1, 'alpha'),
    key('KeyV', 'V', 1, 'alpha'),
    key('KeyB', 'B', 1, 'alpha'),
    key('KeyN', 'N', 1, 'alpha'),
    key('KeyM', 'M', 1, 'alpha'),
    key('Comma', ',', 1, 'alpha'),
    key('Period', '.', 1, 'alpha'),
    key('Slash', '/', 1, 'alpha'),
    key('ShiftRight', 'shift', 1.75, 'mod'),
    key('ArrowUp', '↑', 1, 'arrow'),
    key('End', 'end', 1, 'mod'),
  ],
  [
    key('Fn', 'fn', 1.25, 'mod'),
    key('ControlLeft', 'control', 1.25, 'mod'),
    key('AltLeft', 'option', 1.25, 'mod'),
    key('MetaLeft', 'command', 1.5, 'mod'),
    key('Space', ' ', 5.25, 'space'),
    key('MetaRight', 'command', 1.25, 'mod'),
    key('AltRight', 'option', 1.25, 'mod'),
    key('ArrowLeft', '←', 1, 'arrow'),
    key('ArrowDown', '↓', 1, 'arrow'),
    key('ArrowRight', '→', 1, 'arrow'),
  ],
]

export const KEY_CODES: Set<string> = new Set()

for (const row of MAIN_ROWS) {
  for (const slot of row) {
    if (!isGap(slot)) KEY_CODES.add(slot.code)
  }
}

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
    y: 1,
    scaleY: 0.98,
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
