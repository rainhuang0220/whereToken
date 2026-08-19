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

export function derivationCaption(derivation: string): string {
  const parts = derivation
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
    .map((d) => {
      switch (d) {
        case 'raw':
          return '原始字段'
        case 'provider_api':
          return '账号接口'
        case 'derived':
          return '推导值'
        case 'deduplicated':
          return '按请求去重'
        case 'estimated':
          return '估算'
        default:
          return d
      }
    })
  return parts.join(' · ')
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

function usableCostUSD(usd?: string): string {
  const s = (usd || '').trim()
  if (!s || s === '$0.0000' || s === '-$0.0000' || s === '$0.00' || s === '$0') {
    return ''
  }
  return s
}

export function costCaption(view: Pick<SliceView, 'cost_usd' | 'cost_status' | 'total'>): string {
  const usd = usableCostUSD(view.cost_usd)
  if (usd) {
    return view.cost_status === 'partial' ? `${usd} · 部分` : usd
  }
  if (view.cost_status === 'unavailable' && view.total > 0) {
    return '—'
  }
  return ''
}

export function costKPI(view: Pick<SliceView, 'cost_usd' | 'cost_status' | 'total'>): string {
  const usd = usableCostUSD(view.cost_usd)
  if (usd) return usd
  return '—'
}

export function costHonestyNote(view: Pick<SliceView, 'cost_usd' | 'cost_status' | 'total'>): string {
  const usd = usableCostUSD(view.cost_usd)
  switch (view.cost_status) {
    case 'complete':
      return usd ? `估价 ${usd} · API 标价等价，不是订阅账单` : ''
    case 'partial':
      return usd ? `估价 ${usd} · 部分无标价 · API 标价等价，不是订阅账单` : ''
    case 'unavailable':
      return view.total > 0 ? '估价不可用 · 不会写成 $0' : ''
    default:
      return ''
  }
}

const MIN_RANK_PARTICIPANTS = 20
const displayCrowd = /#\d+\s*\/\s*(\d+)/

export function rankCaption(st?: { status?: string; display?: string; rank?: number }): string {
  if (st?.status && st.status !== 'ok') {
    return '—'
  }
  const crowd = st?.display?.match(displayCrowd)
  if (crowd) {
    const n = Number(crowd[1])
    if (Number.isFinite(n) && n > 0 && n < MIN_RANK_PARTICIPANTS) {
      return '—'
    }
  }
  if (st?.rank && st.rank > 0 && st.display && !st.display.includes('#0')) {
    return st.display
  }
  return '—'
}
