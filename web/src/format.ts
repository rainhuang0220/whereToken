import type { SliceView } from './types'

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
