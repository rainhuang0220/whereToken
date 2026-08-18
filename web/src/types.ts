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
  records?: number
  derivation?: string
  quality: string
  error?: string
}

export type WhyRow = {
  source: string
  label: string
  records: number
  miss: number
  cache_read: number
  cache_create: number
  output: number
  total: number
  quality: string
  derivation: string
}

export type CompareSlice = {
  id: string
  label: string
  total: number
  previous: number
  delta_pct: number | null
  delta_text: string
}

export type CompareView = {
  previous_total: number
  delta_pct: number | null
  by_source: CompareSlice[]
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
  scanned_at?: string
  offline?: boolean
  scanning?: boolean
  all: SliceView
  by_source: SliceView[]
  by_vendor: SliceView[]
  by_source_vendor: SourceVendorView[]
  calendar?: Calendar
  errors: string[]
  drill?: Drill
  why?: WhyRow[]
  compare?: CompareView
  by_model?: SliceView[]
  by_workspace?: SliceView[]
  by_session?: SessionView[]
}

export type PeriodId = 'all' | 'today' | '7d' | '30d'

export type SessionView = SliceView & {
  source: string
  vendor: string
  model: string
  workspace: string
  last_date: string
}

export type DrillTables = {
  models: SliceView[]
  workspaces: SliceView[]
  sessions: SessionView[]
}

export type Drill = {
  all: DrillTables
  by_source: Record<string, DrillTables>
  by_vendor: Record<string, DrillTables>
}

export type Calendar = {
  week_start: 'monday'
  timezone: string
  window_from: string
  window_to: string
  all: CalendarSeries
  by_source: Record<string, CalendarSeries>
  by_vendor: Record<string, CalendarSeries>
}

export type CalendarSeries = {
  days: Day[]
  stats: CalendarStats
}

export type Day = {
  date: string
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
  level: number
}

export type CalendarStats = {
  peak_date: string
  peak_total: number
  peak_total_m: string
  current_streak: number
  longest_streak: number
}

export type AxisKind = 'all' | 'source' | 'vendor'

export type AxisSel = {
  kind: AxisKind
  id: string
}
