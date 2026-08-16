<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AxisDamper from '../components/AxisDamper.vue'
import DrillPanel from '../components/DrillPanel.vue'
import FiringVeil from '../components/FiringVeil.vue'
import FoundryMarks from '../components/FoundryMarks.vue'
import KilnWall from '../components/KilnWall.vue'
import KpiRow from '../components/KpiRow.vue'
import SliceTable from '../components/SliceTable.vue'
import { formatCount } from '../format'
import {
  observatoryCursorWindowHint,
  observatoryDegradedLines,
  observatoryEmptyHint,
  observatoryHasDrill,
  observatoryHasSlice,
  observatoryScanErrorHint,
} from '../observatory'
import { selectDrill, selectSeries, todayISO, wallCells } from '../grid'
import { useSummaryStore } from '../stores/summary'
import type { AxisSel, CalendarSeries, DrillTables } from '../types'

const store = useSummaryStore()
const payload = computed(() => store.payload)
const claudeDegraded = computed(() =>
  payload.value?.by_source?.some((s) => s.id === 'claude' && s.quality === 'degraded'),
)
const cursorAbsent = computed(() =>
  payload.value?.by_source?.some((s) => s.id === 'cursor' && s.quality === 'absent'),
)
const cursorDegraded = computed(() =>
  payload.value?.by_source?.some((s) => s.id === 'cursor' && s.quality === 'degraded'),
)
const traeAbsent = computed(() =>
  payload.value?.by_source?.some((s) => s.id === 'trae' && s.quality === 'absent'),
)
const traeDegraded = computed(() =>
  payload.value?.by_source?.some((s) => s.id === 'trae' && s.quality === 'degraded'),
)
const loginHint = computed(() => {
  const errs = payload.value?.errors ?? []
  const expired = (id: string) =>
    errs.some(
      (e) =>
        e.startsWith(`${id}:`) && (e.includes('已失效') || e.includes('未找到本机登录态')),
    )
  const hasRowError = (id: string) =>
    payload.value?.by_source?.some((s) => s.id === id && Boolean(s.error))
  const names: string[] = []
  if (!traeAbsent.value && !hasRowError('trae') && (expired('trae') || traeDegraded.value)) {
    names.push('Trae')
  }
  if (!cursorAbsent.value && !hasRowError('cursor') && (expired('cursor') || cursorDegraded.value)) {
    names.push('Cursor')
  }
  if (!names.length) return ''
  return `${names.join(' / ')} 需要 IDE 已登录。登录后点「刷新」重新煅烧本机账本；浏览器重载只会显示上次结果。`
})
const degradedLines = computed(() =>
  payload.value ? observatoryDegradedLines(payload.value) : [],
)
const emptyHint = computed(() => (payload.value ? observatoryEmptyHint(payload.value) : ''))
const showSlices = computed(() => (payload.value ? observatoryHasSlice(payload.value) : false))
const showDrill = computed(() => observatoryHasDrill(drill.value))
const cursorWindowHint = computed(() =>
  payload.value ? observatoryCursorWindowHint(payload.value) : '',
)
const scanErrorHint = computed(() => observatoryScanErrorHint(store.error, Boolean(payload.value)))
const offlineBanner = computed(() =>
  payload.value?.offline ? 'offline · 只用本机账本，没有请求 Cursor/Trae 云端' : '',
)
const axis = ref<AxisSel>({ kind: 'all', id: 'all' })

const series = computed<CalendarSeries>(() => selectSeries(payload.value, axis.value))

const cells = computed(() => wallCells(payload.value, axis.value, todayISO()))
const drill = computed<DrillTables>(() => selectDrill(payload.value, axis.value))

const litDays = computed(() => series.value.days.length)
const summaryText = computed(() => {
  const s = series.value.stats
  return `过去 53 周有 ${litDays.value} 天烧过 token，峰值 ${s.peak_date || '—'} ${s.peak_total_m}，当前连烧 ${s.current_streak} 天，最长 ${s.longest_streak} 天。`
})

onMounted(() => {
  void store.hydrate()
})
</script>

<template>
  <div class="forge">
    <header class="rail">
      <h1>whereToken</h1>
      <div class="rail-meta">
        <p class="when">{{ store.scannedAt || '尚未扫描' }}</p>
        <div class="rail-actions">
          <router-link class="lever" to="/themes">主题</router-link>
          <button
            type="button"
            class="lever"
            :class="{ busy: store.loading }"
            :disabled="store.loading"
            :aria-busy="store.loading"
            @click="store.refresh()"
          >
            {{ store.loading ? '煅烧中…' : '刷新' }}
          </button>
        </div>
      </div>
    </header>

    <p v-if="store.error" class="err">{{ store.error }}</p>
    <p v-if="scanErrorHint" class="note">{{ scanErrorHint }}</p>
    <p v-if="offlineBanner" class="note">{{ offlineBanner }}</p>
    <p v-if="emptyHint" class="note">{{ emptyHint }}</p>
    <p v-if="cursorWindowHint" class="note">{{ cursorWindowHint }}</p>
    <p v-if="claudeDegraded" class="note">Claude Code 日志为降级质量：同一请求取最大值，输入/输出可能偏低。</p>
    <p v-if="cursorAbsent" class="note">检测到 Cursor 目录，但没有可读的 state.vscdb 账本。</p>
    <p v-if="traeAbsent" class="note">检测到 Trae 目录，但没有可读的用量账本。</p>
    <p v-for="line in degradedLines" :key="line" class="note">{{ line }}</p>
    <p v-if="loginHint" class="note">{{ loginHint }}</p>

    <AxisDamper
      v-if="payload"
      v-model="axis"
      :sources="payload.by_source"
      :vendors="payload.by_vendor"
    />

    <section class="hearth" :class="{ firing: store.loading }">
      <div class="hearth-stage">
        <KilnWall :cells="cells" :peak-date="series.stats.peak_date" />
        <FiringVeil v-if="store.loading" :progress="store.progress" />
      </div>
      <FoundryMarks :stats="series.stats" />
    </section>
    <p class="sr-only">{{ summaryText }}</p>

    <template v-if="payload">
      <KpiRow :all="payload.all" />

      <div v-if="showSlices" class="split">
        <SliceTable
          title="按工具"
          :rows="payload.by_source"
          :show-turns="true"
          :active-id="axis.kind === 'source' ? axis.id : ''"
          @select="axis = { kind: 'source', id: $event }"
        />
        <SliceTable
          title="按厂家"
          :rows="payload.by_vendor"
          :active-id="axis.kind === 'vendor' ? axis.id : ''"
          @select="axis = { kind: 'vendor', id: $event }"
        />
      </div>

      <DrillPanel v-if="showDrill" :pack="drill" />

      <details v-if="payload.by_source_vendor?.length" class="cross">
        <summary>工具 × 厂家</summary>
        <table>
          <thead>
            <tr>
              <th class="name">工具</th>
              <th class="name">厂家</th>
              <th class="num">未命中</th>
              <th class="num">缓存读</th>
              <th class="num">缓存写</th>
              <th class="num">输出</th>
              <th class="num">合计</th>
              <th class="num">请求</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in payload.by_source_vendor" :key="row.source + row.vendor">
              <td class="name">{{ row.source_label }}</td>
              <td class="name">{{ row.vendor_label }}</td>
              <td class="num">{{ row.miss_m }}</td>
              <td class="num">{{ row.cache_read_m }}</td>
              <td class="num">{{ row.cache_create_m }}</td>
              <td class="num">{{ row.output_m }}</td>
              <td class="num">{{ row.total_m }}</td>
              <td class="num">{{ formatCount(row.requests) }}</td>
            </tr>
          </tbody>
        </table>
      </details>

      <p v-if="payload.errors.length" class="err">{{ payload.errors.join(' · ') }}</p>
    </template>
  </div>
</template>
