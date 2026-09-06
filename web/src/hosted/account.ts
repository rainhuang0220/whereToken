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

export function moveMenuIndex(items: MenuItem[], index: number, delta: number): number {
  const ids = items.map((it, i) => (it.kind === 'item' ? i : -1)).filter((i) => i >= 0)
  if (!ids.length) return 0
  const pos = ids.indexOf(index)
  const next = pos < 0 ? ids[0] : ids[(pos + delta + ids.length) % ids.length]
  return next
}
