<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AxisDamper from '../components/AxisDamper.vue'
import DrillPanel from '../components/DrillPanel.vue'
import FoundryMarks from '../components/FoundryMarks.vue'
import KilnWall from '../components/KilnWall.vue'
import KpiRow from '../components/KpiRow.vue'
import SliceTable from '../components/SliceTable.vue'
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
  void store.refresh()
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
          <button type="button" class="lever" :disabled="store.loading" @click="store.refresh()">
            {{ store.loading ? '煅烧中…' : '刷新' }}
          </button>
        </div>
      </div>
    </header>

    <p v-if="store.error" class="err">{{ store.error }}</p>
    <p v-if="claudeDegraded" class="note">Claude Code 日志为降级质量：同一请求取最大值，输入/输出可能偏低。</p>
    <p v-if="cursorAbsent" class="note">检测到 Cursor 目录，但没有可读的 state.vscdb 账本。</p>
    <p v-else-if="cursorDegraded" class="note">
      Cursor 已计入本机请求与回合。token 列若为 0，是因为没有拉到 Cursor 账号用量（见下方 errors），不是没扫到。
    </p>
    <p v-if="traeAbsent" class="note">检测到 Trae 目录，但没有可读的用量账本。</p>
    <p v-else-if="traeDegraded" class="note">
      Trae 已发现本机会话。token 列若为 0，是因为没有拉到 Trae 账号用量（见下方 errors），不是没扫到。
    </p>

    <template v-if="payload">
      <AxisDamper
        v-model="axis"
        :sources="payload.by_source"
        :vendors="payload.by_vendor"
      />

      <section class="hearth">
        <KilnWall :cells="cells" :peak-date="series.stats.peak_date" />
        <FoundryMarks :stats="series.stats" />
      </section>
      <p class="sr-only">{{ summaryText }}</p>

      <KpiRow :all="payload.all" />

      <div class="split">
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

      <DrillPanel :pack="drill" />

      <details class="cross">
        <summary>工具 × 厂家</summary>
        <table>
          <thead>
            <tr>
              <th class="name">工具</th>
              <th class="name">厂家</th>
              <th class="num">未命中</th>
              <th class="num">缓存读</th>
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
              <td class="num">{{ row.output_m }}</td>
              <td class="num">{{ row.total_m }}</td>
              <td class="num">{{ row.requests }}</td>
            </tr>
          </tbody>
        </table>
      </details>

      <p v-if="payload.errors.length" class="err">{{ payload.errors.join(' · ') }}</p>
    </template>
  </div>
</template>
