import { describe, expect, it } from 'vitest'
import { readScanStream } from './api'

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
})
