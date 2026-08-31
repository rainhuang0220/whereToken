import type { SummaryPayload } from './types'
import { isDemo } from './demo'
import { parseSSEBlock, scanEventError, splitSSE, type ScanProgress } from './firing'

export async function waitWhileScanning(opts?: {
  fetchImpl?: typeof fetch
  intervalMs?: number
  timeoutMs?: number
  now?: () => number
  sleep?: (ms: number) => Promise<void>
}): Promise<SummaryPayload> {
  const doFetch = opts?.fetchImpl ?? fetch
  const interval = opts?.intervalMs ?? 200
  const timeout = opts?.timeoutMs ?? 60_000
  const now = opts?.now ?? Date.now
  const sleep = opts?.sleep ?? ((ms: number) => new Promise((resolve) => setTimeout(resolve, ms)))
  const start = now()
  while (now() - start < timeout) {
    const res = await doFetch('/api/summary', { cache: 'no-store' })
    if (!res.ok) {
      throw new Error(`summary ${res.status}`)
    }
    const payload = (await res.json()) as SummaryPayload
    if (!payload.scanning && payload.scanned_at) {
      return payload
    }
    await sleep(interval)
  }
  throw new Error('煅烧进行中')
}

export async function setCommunity(enabled: boolean): Promise<void> {
  if (isDemo()) return
  const res = await fetch('/api/community', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    cache: 'no-store',
    body: JSON.stringify({ enabled }),
  })
  if (!res.ok) {
    throw new Error(`community ${res.status}`)
  }
}

export async function fetchSummary(since?: string): Promise<SummaryPayload> {
  if (isDemo()) {
    const period = since && since !== 'all' ? since : 'all'
    const res = await fetch(`${import.meta.env.BASE_URL}sample/${period}.json`, {
      cache: 'no-store',
    })
    if (!res.ok) {
      throw new Error(`sample ${res.status}`)
    }
    return res.json() as Promise<SummaryPayload>
  }
  const q = since && since !== 'all' ? `?since=${encodeURIComponent(since)}` : ''
  const res = await fetch(`/api/summary${q}`, { cache: 'no-store' })
  if (!res.ok) {
    throw new Error(`summary ${res.status}`)
  }
  return res.json() as Promise<SummaryPayload>
}

export async function rescan(onProgress: (p: ScanProgress) => void): Promise<SummaryPayload> {
  if (isDemo()) {
    return fetchSummary('all')
  }
  const res = await fetch('/api/scan', {
    method: 'POST',
    headers: { Accept: 'text/event-stream' },
    cache: 'no-store',
  })
  if (res.status === 409) {
    return waitWhileScanning()
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
        try {
          onProgress(JSON.parse(ev.data) as ScanProgress)
        } catch {
          // skip malformed progress frames
        }
      }
      if (ev.event === 'complete' && ev.data) {
        try {
          complete = JSON.parse(ev.data) as SummaryPayload
        } catch {
          // fall through to the 'scan incomplete' guard below
        }
      }
    }
    if (done) break
  }
  if (!complete) {
    throw new Error('scan incomplete')
  }
  return complete
}
