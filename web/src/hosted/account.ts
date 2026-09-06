export const UNAUTHORIZED = 'unauthorized'

export function isUnauthorized(err: unknown): boolean {
  return err instanceof Error && err.message === UNAUTHORIZED
}

export function safeReturnPath(raw: string | null | undefined): string {
  const p = String(raw || '').trim()
  if (!p.startsWith('/') || p.startsWith('//') || p.includes('://') || p.includes('\\')) {
    return '/app'
  }
  return p
}

export function displayHandle(login: string): string {
  const name = login.trim()
  if (!name) return ''
  return name.startsWith('@') ? name : `@${name}`
}

export function avatarInitial(login: string): string {
  const name = login.trim().replace(/^@/, '')
  const ch = name.charAt(0)
  return ch ? ch.toUpperCase() : '?'
}

export type MenuItem = { id: string; kind: 'item' | 'sep' }

export function menuItemIndexes(items: MenuItem[]): number[] {
  return items.map((it, i) => (it.kind === 'item' ? i : -1)).filter((i) => i >= 0)
}

export function moveMenuIndex(items: MenuItem[], index: number, delta: number): number {
  const ids = menuItemIndexes(items)
  if (!ids.length) return 0
  const pos = ids.indexOf(index)
  const next = pos < 0 ? ids[0] : ids[(pos + delta + ids.length) % ids.length]
  return next
}

export function firstMenuIndex(items: MenuItem[]): number {
  return menuItemIndexes(items)[0] ?? 0
}

export function lastMenuIndex(items: MenuItem[]): number {
  const ids = menuItemIndexes(items)
  return ids.length ? ids[ids.length - 1] : 0
}

export type TriggerMenuAction = 'open-first' | 'open-last' | 'toggle' | 'close' | ''

export function triggerMenuKey(open: boolean, key: string): TriggerMenuAction {
  if (key === 'Escape') return open ? 'close' : ''
  if (key === 'ArrowDown') return 'open-first'
  if (key === 'ArrowUp') return 'open-last'
  if (key === 'Enter' || key === ' ') return 'toggle'
  return ''
}

export function menuOffset(
  trigger: { top: number; left: number; width: number; height: number },
  menu: { width: number; height: number },
  viewport: { width: number; height: number },
  gap = 4,
): { top: number; left: number } {
  let top = trigger.height + gap
  let left = trigger.width - menu.width
  const absTop = trigger.top + top
  const absLeft = trigger.left + left
  if (absTop + menu.height > viewport.height - 8) {
    top = -menu.height - gap
  }
  if (absLeft < 8) {
    left = 8 - trigger.left
  }
  if (trigger.left + left + menu.width > viewport.width - 8) {
    left = viewport.width - 8 - menu.width - trigger.left
  }
  return { top, left }
}
