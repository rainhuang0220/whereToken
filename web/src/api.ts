import type { SummaryPayload } from './types'

export async function fetchSummary(): Promise<SummaryPayload> {
  const res = await fetch('/api/summary')
  if (!res.ok) {
    throw new Error(`summary ${res.status}`)
  }
  return res.json() as Promise<SummaryPayload>
}
