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

export function columnsFrom(view: SliceView): string[] {
  return [
    view.miss_m,
    view.cache_read_m,
    view.cache_create_m,
    view.output_m,
    view.total_m,
    view.hit_rate_text,
  ]
}
