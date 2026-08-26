import { describe, expect, it } from 'vitest'
import { readScanStream, waitWhileScanning } from './api'

function streamOf(text: string): ReadableStream<Uint8Array> {
  return new ReadableStream({
    start(controller) {
      controller.enqueue(new TextEncoder().encode(text))
      controller.close()
    },
  })
}

describe('readScanStream', () => {
  it('throws the server error event instead of scan incomplete', async () => {
    await expect(
      readScanStream(streamOf('event: error\ndata: {"error":"encode"}\n\n'), () => {}),
    ).rejects.toThrow('encode')
  })

  it('returns the complete payload on the real SSE path', async () => {
    const payload = await readScanStream(
      streamOf(
        'event: progress\ndata: {"source":"kimi","label":"正在读 Kimi Code…","index":1,"total":1,"status":"reading"}\n\nevent: complete\ndata: {"all":{"total":1185,"total_m":"0.0012 M"},"scanned_at":"2026-08-17T02:00:00+08:00"}\n\n',
      ),
      () => {},
    )
    expect(payload.all.total).toBe(1185)
    expect(payload.scanned_at).toContain('2026-08-17')
  })

  it('skips a malformed progress frame instead of aborting the scan', async () => {
    const seen: string[] = []
    const payload = await readScanStream(
      streamOf(
        'event: progress\ndata: {oops\n\nevent: complete\ndata: {"all":{"total":1},"scanned_at":"2026-08-17T02:00:00+08:00"}\n\n',
      ),
      (p) => seen.push(p.label),
    )
    expect(seen).toEqual([])
    expect(payload.all.total).toBe(1)
  })

  it('surfaces a malformed complete payload as scan incomplete', async () => {
    await expect(
      readScanStream(streamOf('event: complete\ndata: {oops\n\n'), () => {}),
    ).rejects.toThrow('scan incomplete')
  })
})

describe('waitWhileScanning', () => {
  it('returns the summary once scanning flips off', async () => {
    let n = 0
    const payload = await waitWhileScanning({
      intervalMs: 0,
      timeoutMs: 1000,
      now: (() => {
        let t = 0
        return () => (t += 1)
      })(),
      sleep: async () => {},
      fetchImpl: async () => {
        n += 1
        const body =
          n < 3
            ? { scanning: true }
            : { scanning: false, scanned_at: '2026-08-17T03:00:00+08:00', all: { total: 1 } }
        return new Response(JSON.stringify(body), { status: 200 })
      },
    })
    expect(n).toBe(3)
    expect(payload.scanned_at).toContain('2026-08-17')
  })

  it('gives up with 煅烧进行中 when the other scan never finishes', async () => {
    await expect(
      waitWhileScanning({
        intervalMs: 0,
        timeoutMs: 2,
        now: (() => {
          let t = 0
          return () => (t += 1)
        })(),
        sleep: async () => {},
        fetchImpl: async () => new Response(JSON.stringify({ scanning: true }), { status: 200 }),
      }),
    ).rejects.toThrow('煅烧进行中')
  })
})
