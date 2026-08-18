import type { SliceView } from './types'

export function formatCount(n: number): string {
  const neg = n < 0
  const digits = Math.abs(Math.trunc(n)).toString()
  let grouped = ''
  for (let i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 === 0) {
      grouped += ','
    }
    grouped += digits[i]
  }
  return neg ? `-${grouped}` : grouped
}

export function hitBand(text: string): 'hi' | 'mid' | 'lo' | 'none' {
  const raw = text.trim()
  if (!raw || raw === '—') return 'none'
  const pct = Number.parseFloat(raw)
  if (!Number.isFinite(pct)) return 'none'
  if (pct >= 70) return 'hi'
  if (pct >= 40) return 'mid'
  return 'lo'
}

export function qualityCaption(quality: string): string {
  switch (quality) {
    case 'authoritative':
      return '完整'
    case 'degraded':
      return '降级'
    case 'estimated':
      return '估算'
    case 'absent':
      return '数据不可用'
    default:
      return ''
  }
}

export function tokenCell(formatted: string, quality: string): string {
  if (quality === 'absent') {
    return '不可用'
  }
  return formatted
}

export function columnsFrom(view: SliceView): string[] {
  return [
    tokenCell(view.miss_m, view.quality),
    tokenCell(view.cache_read_m, view.quality),
    tokenCell(view.cache_create_m, view.quality),
    tokenCell(view.output_m, view.quality),
    tokenCell(view.total_m, view.quality),
    view.quality === 'absent' ? '—' : view.hit_rate_text,
  ]
}
