export type ScanProgress = {
  source: string
  label: string
  index: number
  total: number
  status: 'reading' | 'done' | 'error'
}

export type SSEEvent = {
  event: string
  data: string
}

export function chargeAmount(p: ScanProgress | null | undefined): number {
  if (!p || p.total <= 0) return 0
  if (p.status === 'reading') return Math.max(0, p.index - 1) / p.total
  return Math.min(1, p.index / p.total)
}

export function splitSSE(raw: string): { events: SSEEvent[]; rest: string } {
  const parts = raw.split('\n\n')
  const rest = parts.pop() ?? ''
  const events: SSEEvent[] = []
  for (const block of parts) {
    let event = 'message'
    let data = ''
    for (const line of block.split('\n')) {
      if (line.startsWith('event:')) event = line.slice(6).trim()
      if (line.startsWith('data:')) data = line.slice(5).trimStart()
    }
    if (event !== 'message' || data) events.push({ event, data })
  }
  return { events, rest }
}

export function parseSSEBlock(raw: string): SSEEvent[] {
  const padded = raw.endsWith('\n\n') ? raw : `${raw}\n\n`
  return splitSSE(padded).events
}

export function formatScannedAt(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN')
}
