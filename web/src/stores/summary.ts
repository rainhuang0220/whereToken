import { defineStore } from 'pinia'
import { fetchSummary, rescan } from '../api'
import { formatScannedAt, type ScanProgress } from '../firing'
import type { SummaryPayload } from '../types'

export const useSummaryStore = defineStore('summary', {
  state: () => ({
    payload: null as SummaryPayload | null,
    error: '',
    loading: false,
    scannedAt: '',
    progress: null as ScanProgress | null,
  }),
  actions: {
    async hydrate() {
      try {
        const last = await fetchSummary()
        if (last.scanned_at) {
          this.payload = last
          this.scannedAt = formatScannedAt(last.scanned_at)
        }
      } catch (err) {
        this.error = err instanceof Error ? err.message : String(err)
      }
      if (!this.payload?.scanned_at) {
        await this.refresh()
      }
    },
    async refresh() {
      if (this.loading) return
      this.loading = true
      this.error = ''
      this.progress = {
        source: '',
        label: '正在读本机账本…',
        index: 1,
        total: 1,
        status: 'reading',
      }
      try {
        const next = await rescan((p) => {
          this.progress = p
        })
        this.payload = next
        this.scannedAt = formatScannedAt(next.scanned_at) || new Date().toLocaleString('zh-CN')
      } catch (err) {
        this.error = err instanceof Error ? err.message : String(err)
      } finally {
        this.loading = false
      }
    },
  },
})
