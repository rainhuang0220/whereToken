import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fetchSummary, rescan } from '../api'
import type { SliceView, SummaryPayload } from '../types'
import { useSummaryStore } from './summary'

vi.mock('../api', () => ({
  fetchSummary: vi.fn(),
  rescan: vi.fn(),
  setCommunity: vi.fn(),
}))

const fetchSummaryMock = vi.mocked(fetchSummary)
const rescanMock = vi.mocked(rescan)

function slice(total: number): SliceView {
  return {
    id: 'all',
    label: '合计',
    miss: 0,
    cache_read: 0,
    cache_create: 0,
    output: 0,
    total,
    miss_m: '0.00 M',
    cache_read_m: '0.00 M',
    cache_create_m: '0.00 M',
    output_m: '0.00 M',
    total_m: '0.00 M',
    hit_rate: null,
    hit_rate_text: '—',
    requests: 0,
    user_turns: 0,
    quality: 'authoritative',
  }
}

function payload(total: number): SummaryPayload {
  return {
    scanned_at: '2026-08-17T02:00:00+08:00',
    all: slice(total),
    by_source: [],
    by_vendor: [],
    by_source_vendor: [],
    errors: [],
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (err: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('summary store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('setPeriod during the first hydrate still lands a payload', async () => {
    const first = deferred<SummaryPayload>()
    const windowed = deferred<SummaryPayload>()
    fetchSummaryMock.mockImplementation((since?: string) =>
      since === '7d' ? windowed.promise : first.promise,
    )
    const store = useSummaryStore()
    const hydrating = store.hydrate()
    const switching = store.setPeriod('7d')
    first.resolve(payload(1))
    windowed.resolve(payload(7))
    await Promise.all([hydrating, switching])
    expect(store.period).toBe('7d')
    expect(store.payload?.all.total).toBe(7)
    expect(rescanMock).not.toHaveBeenCalled()
  })

  it('keeps the old period label when the windowed fetch fails', async () => {
    const store = useSummaryStore()
    store.payload = payload(100)
    fetchSummaryMock.mockRejectedValue(new Error('summary 500'))
    await store.setPeriod('7d')
    expect(store.period).toBe('all')
    expect(store.payload?.all.total).toBe(100)
    expect(store.error).toBe('summary 500')
  })

  it('clears progress once a refresh settles', async () => {
    rescanMock.mockImplementation(async (onProgress) => {
      onProgress({ source: 'kimi', label: '正在读 Kimi Code…', index: 1, total: 1, status: 'reading' })
      return payload(42)
    })
    const store = useSummaryStore()
    await store.refresh()
    expect(store.loading).toBe(false)
    expect(store.progress).toBeNull()
    expect(store.payload?.all.total).toBe(42)
  })

  it('clears progress when a refresh fails', async () => {
    rescanMock.mockRejectedValue(new Error('scan 500'))
    const store = useSummaryStore()
    await store.refresh()
    expect(store.loading).toBe(false)
    expect(store.progress).toBeNull()
    expect(store.error).toBe('scan 500')
  })
})
