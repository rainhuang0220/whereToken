import type { SummaryPayload } from './types'
import { parseSSEBlock, scanEventError, splitSSE, type ScanProgress } from './firing'

export async function fetchSummary(): Promise<SummaryPayload> {
  const res = await fetch('/api/summary', { cache: 'no-store' })
  if (!res.ok) {
    throw new Error(`summary ${res.status}`)
  }
  return res.json() as Promise<SummaryPayload>
}

export async function rescan(onProgress: (p: ScanProgress) => void): Promise<SummaryPayload> {
  const res = await fetch('/api/scan', {
    method: 'POST',
    headers: { Accept: 'text/event-stream' },
    cache: 'no-store',
  })
  if (res.status === 409) {
    throw new Error('煅烧进行中')
  }
  if (!res.ok) {
    throw new Error(`scan ${res.status}`)
  }
  const ctype = res.headers.get('content-type') || ''
  if (ctype.includes('text/event-stream') && res.body) {
    return readScanStream(res.body, onProgress)
  }
  return res.json() as Promise<SummaryPayload>
}

export async function readScanStream(
  body: ReadableStream<Uint8Array>,
  onProgress: (p: ScanProgress) => void,
): Promise<SummaryPayload> {
  const reader = body.getReader()
  const dec = new TextDecoder()
  let buf = ''
  let complete: SummaryPayload | null = null
  while (true) {
    const { done, value } = await reader.read()
    buf += dec.decode(value || new Uint8Array(), { stream: !done })
    const split = done ? { events: parseSSEBlock(buf), rest: '' } : splitSSE(buf)
    buf = split.rest
    for (const ev of split.events) {
      const err = scanEventError(ev)
      if (err) {
        throw new Error(err)
      }
      if (ev.event === 'progress' && ev.data) {
        onProgress(JSON.parse(ev.data) as ScanProgress)
      }
      if (ev.event === 'complete' && ev.data) {
        complete = JSON.parse(ev.data) as SummaryPayload
      }
    }
    if (done) break
  }
  if (!complete) {
    throw new Error('scan incomplete')
  }
  return complete
}
