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

export function brickCaption(cell: Cell): { date: string; amount: string } {
  const parts = cell.date.split('-')
  const date = `${Number(parts[1])}月${Number(parts[2])}日`
  if (cell.kind === 'future') return { date, amount: '未到' }
  if (cell.kind === 'empty' || !cell.day) return { date, amount: '0.00 M' }
  return { date, amount: cell.day.total_m }
}

export function brickAriaLabel(cell: Cell): string {
  const cap = brickCaption(cell)
  return `${cap.date} ${cap.amount}`
}

export function kilnStep(i: number, key: string, n: number): number {
  if (n <= 0 || i < 0 || i >= n) return i
  const weekday = i % 7
  switch (key) {
    case 'ArrowDown':
      return weekday < 6 && i + 1 < n ? i + 1 : i
    case 'ArrowUp':
      return weekday > 0 ? i - 1 : i
    case 'ArrowRight':
      return i + 7 < n ? i + 7 : i
    case 'ArrowLeft':
      return i - 7 >= 0 ? i - 7 : i
    case 'Home':
      return i - weekday
    case 'End': {
      const end = i - weekday + 6
      return end < n ? end : n - 1
    }
    default:
      return i
  }
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
