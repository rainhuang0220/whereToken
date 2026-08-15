import type { SliceView, SummaryPayload } from './types'

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
