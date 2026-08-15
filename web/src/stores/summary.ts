import { defineStore } from 'pinia'
import { fetchSummary } from '../api'
import type { SummaryPayload } from '../types'

export const useSummaryStore = defineStore('summary', {
  state: () => ({
    payload: null as SummaryPayload | null,
    error: '',
    loading: false,
    scannedAt: '',
  }),
  actions: {
    async refresh() {
      this.loading = true
      this.error = ''
      try {
        this.payload = await fetchSummary()
        this.scannedAt = new Date().toLocaleString('zh-CN')
      } catch (err) {
        this.error = err instanceof Error ? err.message : String(err)
      } finally {
        this.loading = false
      }
    },
  },
})
