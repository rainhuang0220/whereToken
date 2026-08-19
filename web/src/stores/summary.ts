import { defineStore } from 'pinia'
import { fetchSummary, rescan, setCommunity } from '../api'
import { formatScannedAt, type ScanProgress } from '../firing'
import { acceptPeriod } from '../periodSeq'
import type { PeriodId, RankPeriod, SummaryPayload } from '../types'

export const useSummaryStore = defineStore('summary', {
  state: () => ({
    payload: null as SummaryPayload | null,
    error: '',
    loading: false,
    scannedAt: '',
    progress: null as ScanProgress | null,
    period: 'all' as PeriodId,
    periodSeq: 0,
    rankPeriod: 'today' as RankPeriod,
  }),
  actions: {
    async hydrate() {
      const n = ++this.periodSeq
      const want = this.period
      try {
        const last = await fetchSummary(want)
        if (!acceptPeriod(this.periodSeq, n)) return
        if (last.scanned_at) {
          this.payload = last
          this.scannedAt = formatScannedAt(last.scanned_at)
        }
      } catch (err) {
        if (!acceptPeriod(this.periodSeq, n)) return
        this.error = err instanceof Error ? err.message : String(err)
      }
      if (!acceptPeriod(this.periodSeq, n)) return
      if (!this.payload?.scanned_at) {
        await this.refresh()
      }
    },
    async setPeriod(period: PeriodId) {
      if (this.period === period && this.payload) return
      this.period = period
      const n = ++this.periodSeq
      if (!this.payload?.scanned_at) return
      try {
        const next = await fetchSummary(period)
        if (!acceptPeriod(this.periodSeq, n)) return
        this.payload = next
        this.error = ''
      } catch (err) {
        if (!acceptPeriod(this.periodSeq, n)) return
        this.error = err instanceof Error ? err.message : String(err)
      }
    },
    async refresh() {
      if (this.loading) return
      this.loading = true
      this.error = ''
      const n = ++this.periodSeq
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
        if (!acceptPeriod(this.periodSeq, n)) return
        this.scannedAt = formatScannedAt(next.scanned_at) || new Date().toLocaleString('zh-CN')
        if (this.period === 'all') {
          this.payload = next
        } else {
          const windowed = await fetchSummary(this.period)
          if (!acceptPeriod(this.periodSeq, n)) return
          this.payload = windowed
        }
      } catch (err) {
        if (!acceptPeriod(this.periodSeq, n)) return
        this.error = err instanceof Error ? err.message : String(err)
      } finally {
        this.loading = false
      }
    },
    setRankPeriod(period: RankPeriod) {
      this.rankPeriod = period
    },
    async toggleCommunity(enabled: boolean) {
      try {
        await setCommunity(enabled)
        await this.refresh()
      } catch (err) {
        this.error = err instanceof Error ? err.message : String(err)
      }
    },
  },
})
