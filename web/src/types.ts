export type SliceView = {
  id: string
  label: string
  miss: number
  cache_read: number
  cache_create: number
  output: number
  total: number
  miss_m: string
  cache_read_m: string
  cache_create_m: string
  output_m: string
  total_m: string
  hit_rate: number | null
  hit_rate_text: string
  requests: number
  user_turns: number
  quality: string
}

export type SourceVendorView = {
  source: string
  vendor: string
  source_label: string
  vendor_label: string
  miss: number
  cache_read: number
  cache_create: number
  output: number
  total: number
  miss_m: string
  cache_read_m: string
  cache_create_m: string
  output_m: string
  total_m: string
  requests: number
}

export type SummaryPayload = {
  all: SliceView
  by_source: SliceView[]
  by_vendor: SliceView[]
  by_source_vendor: SourceVendorView[]
  errors: string[]
}
