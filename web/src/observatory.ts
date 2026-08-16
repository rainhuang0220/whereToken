import type { SliceView, SummaryPayload } from './types'

export function observatoryCursorWindowHint(
  payload: Pick<SummaryPayload, 'by_source'>,
): string {
  for (const row of payload.by_source ?? []) {
    if (row.id === 'cursor' && row.quality === 'authoritative' && row.total !== 0) {
      return 'Cursor 的 token 列是近 53 周账号用量；请求和回合仍是本机全部会话。'
    }
  }
  return ''
}

export function observatoryScanErrorHint(error: string, hasPayload: boolean): string {
  const msg = error.trim()
  if (!msg) return ''
  if (msg.includes('409') || msg.includes('煅烧进行中')) {
    return '另一处正在刷新。等这次煅烧结束再点。'
  }
  if (hasPayload) return ''
  return '扫描没完成。看上面的错误后再点「刷新」。'
}

export function observatoryHasSlice(
  payload: Pick<SummaryPayload, 'by_source' | 'by_vendor'>,
): boolean {
  return (payload.by_source?.length ?? 0) > 0 || (payload.by_vendor?.length ?? 0) > 0
}

export function observatoryHasDrill(pack: { models?: unknown[]; workspaces?: unknown[]; sessions?: unknown[] } | null | undefined): boolean {
  if (!pack) return false
  return (pack.models?.length ?? 0) + (pack.workspaces?.length ?? 0) + (pack.sessions?.length ?? 0) > 0
}

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
