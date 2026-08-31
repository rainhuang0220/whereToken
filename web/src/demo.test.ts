import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchSummary, rescan } from './api'

describe('demo mode', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
  })

  it('fetchSummary reads the committed sample payload, not /api', async () => {
    vi.stubEnv('VITE_DEMO', '1')
    const seen: string[] = []
    vi.stubGlobal('fetch', async (url: RequestInfo | URL) => {
      seen.push(String(url))
      return new Response(
        JSON.stringify({ all: { total: 7 }, scanned_at: '2026-08-31T08:00:00+08:00' }),
        { status: 200 },
      )
    })
    const p = await fetchSummary('7d')
    expect(seen).toEqual(['/sample/7d.json'])
    expect(p.all.total).toBe(7)
  })

  it('rescan is inert and returns the all-period sample', async () => {
    vi.stubEnv('VITE_DEMO', '1')
    const seen: string[] = []
    vi.stubGlobal('fetch', async (url: RequestInfo | URL) => {
      seen.push(String(url))
      return new Response(JSON.stringify({ all: { total: 1 }, scanned_at: 'x' }), { status: 200 })
    })
    const p = await rescan(() => {})
    expect(seen).toEqual(['/sample/all.json'])
    expect(p.all.total).toBe(1)
  })

  it('fetchSummary without VITE_DEMO still hits /api', async () => {
    const seen: string[] = []
    vi.stubGlobal('fetch', async (url: RequestInfo | URL) => {
      seen.push(String(url))
      return new Response(JSON.stringify({ all: { total: 1 } }), { status: 200 })
    })
    await fetchSummary('today')
    expect(seen).toEqual(['/api/summary?since=today'])
  })
})
