import type { AxisSel, SliceView } from './types'

export type DamperTab = {
  kind: AxisSel['kind']
  id: string
  label: string
}

export function damperTabs(sources: SliceView[] | undefined, vendors: SliceView[] | undefined): DamperTab[] {
  const tabs: DamperTab[] = [{ kind: 'all', id: 'all', label: '合计' }]
  for (const row of sources ?? []) {
    if (row.quality === 'absent') continue
    tabs.push({ kind: 'source', id: row.id, label: row.label })
  }
  for (const row of vendors ?? []) {
    if (row.quality === 'absent') continue
    tabs.push({ kind: 'vendor', id: row.id, label: row.label })
  }
  return tabs
}

export function damperIndex(tabs: DamperTab[], sel: AxisSel): number {
  const i = tabs.findIndex((t) => t.kind === sel.kind && t.id === sel.id)
  return i < 0 ? 0 : i
}

export function damperStep(i: number, key: string, n: number): number {
  if (n <= 0) return 0
  if (i < 0) i = 0
  if (i >= n) i = n - 1
  switch (key) {
    case 'ArrowRight':
    case 'ArrowDown':
      return Math.min(n - 1, i + 1)
    case 'ArrowLeft':
    case 'ArrowUp':
      return Math.max(0, i - 1)
    case 'Home':
      return 0
    case 'End':
      return n - 1
    default:
      return i
  }
}
