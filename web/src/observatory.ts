import type { SliceView, SummaryPayload } from './types'

export function observatoryEmptyHint(
  payload: Pick<SummaryPayload, 'all' | 'by_source'>,
): string {
  const all = payload.all
  if (!all) return ''
  if (all.total !== 0 || all.requests !== 0 || all.user_turns !== 0) return ''
  if ((payload.by_source ?? []).length > 0) return ''
  return '本机没有找到账本。Claude / Kimi / Codex / OpenCode 有本地记录才会出数；Cursor / Trae 需要已登录。'
}

export function observatoryDegradedLines(
  payload: Pick<SummaryPayload, 'by_source' | 'by_vendor'>,
): string[] {
  const lines: string[] = []
  const seen = new Set<string>()
  for (const row of [...payload.by_source, ...payload.by_vendor]) {
    if (row.quality !== 'degraded') continue
    const reason = row.error?.trim()
    if (!reason) continue
    const line = `${row.label}：${reason}`
    if (seen.has(line)) continue
    seen.add(line)
    lines.push(line)
  }
  return lines
}
