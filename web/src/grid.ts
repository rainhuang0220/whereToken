import type { AxisSel, CalendarSeries, Day, DrillTables, SummaryPayload } from './types'

export type CellKind = 'empty' | 'lit' | 'future'

export type Cell = {
  date: string
  kind: CellKind
  level: number
  day?: Day
  weekday: number
  weekIndex: number
}

function parseISODate(iso: string): Date {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export function formatISODate(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function todayISO(): string {
  return formatISODate(new Date())
}

function mondayIndex(d: Date): number {
  return (d.getDay() + 6) % 7
}

export function layoutCells(opts: {
  windowFrom: string
  windowTo: string
  today: string
  weekStart: 'monday'
  days: Day[]
}): Cell[] {
  const from = parseISODate(opts.windowFrom)
  const today = parseISODate(opts.today)
  const weekEnd = new Date(today)
  weekEnd.setDate(weekEnd.getDate() + (6 - mondayIndex(today)))
  const requestedTo = parseISODate(opts.windowTo)
  const to = requestedTo.getTime() > weekEnd.getTime() ? requestedTo : weekEnd
  const byDate = new Map((opts.days ?? []).map((d) => [d.date, d]))
  const cells: Cell[] = []
  for (let cursor = new Date(from), i = 0; cursor.getTime() <= to.getTime(); cursor.setDate(cursor.getDate() + 1), i++) {
    const date = formatISODate(cursor)
    const hit = byDate.get(date)
    let kind: CellKind = 'empty'
    if (cursor.getTime() > today.getTime()) {
      kind = 'future'
    } else if (hit) {
      kind = 'lit'
    }
    cells.push({
      date,
      kind,
      level: hit?.level ?? 0,
      day: hit,
      weekday: mondayIndex(cursor),
      weekIndex: Math.floor(i / 7),
    })
  }
  return cells
}

export function monthLabels(cells: Cell[]): { weekIndex: number; label: string }[] {
  const seen = new Set<string>()
  const out: { weekIndex: number; label: string }[] = []
  for (const cell of cells) {
    const month = cell.date.slice(0, 7)
    if (seen.has(month)) continue
    seen.add(month)
    const m = Number(cell.date.slice(5, 7))
    out.push({ weekIndex: cell.weekIndex, label: `${m}月` })
  }
  return out
}

export const emptySeries: CalendarSeries = {
  days: [],
  stats: {
    peak_date: '',
    peak_total: 0,
    peak_total_m: '0.00 M',
    current_streak: 0,
    longest_streak: 0,
  },
}

export function selectSeries(payload: SummaryPayload | null | undefined, axis: AxisSel): CalendarSeries {
  const cal = payload?.calendar
  if (!cal) return emptySeries
  if (axis.kind === 'source') return cal.by_source?.[axis.id] ?? emptySeries
  if (axis.kind === 'vendor') return cal.by_vendor?.[axis.id] ?? emptySeries
  return cal.all ?? emptySeries
}

export function defaultWindow(today: string): { from: string; to: string } {
  const d = parseISODate(today)
  const weekStart = new Date(d)
  weekStart.setDate(weekStart.getDate() - mondayIndex(d))
  const from = new Date(weekStart)
  from.setDate(from.getDate() - 52 * 7)
  return { from: formatISODate(from), to: today }
}

export function wallCells(payload: SummaryPayload | null | undefined, axis: AxisSel, today: string): Cell[] {
  const cal = payload?.calendar
  const series = selectSeries(payload, axis)
  const fallback = defaultWindow(today)
  return layoutCells({
    windowFrom: cal?.window_from || fallback.from,
    windowTo: cal?.window_to || fallback.to,
    today,
    weekStart: 'monday',
    days: series.days ?? [],
  })
}

const emptyDrill: DrillTables = { models: [], workspaces: [], sessions: [] }

export function selectDrill(payload: SummaryPayload | null | undefined, axis: AxisSel): DrillTables {
  const drill = payload?.drill
  if (!drill) return emptyDrill
  if (axis.kind === 'source') return drill.by_source?.[axis.id] ?? emptyDrill
  if (axis.kind === 'vendor') return drill.by_vendor?.[axis.id] ?? emptyDrill
  return drill.all ?? emptyDrill
}
