<script setup lang="ts">
import { computed, onMounted } from 'vue'
import KpiRow from './components/KpiRow.vue'
import ShareBars from './components/ShareBars.vue'
import SliceTable from './components/SliceTable.vue'
import { useSummaryStore } from './stores/summary'

const store = useSummaryStore()
const payload = computed(() => store.payload)
const degraded = computed(() => payload.value?.all.quality === 'degraded')

onMounted(() => {
  void store.refresh()
})
</script>

<template>
  <div class="sheet">
    <header class="mast">
      <div>
        <p class="kicker">本机观测</p>
        <h1>whereToken</h1>
        <p class="sub">token 花在哪：合计 · 按工具 · 按厂家</p>
      </div>
      <div class="mast-meta">
        <p>{{ store.scannedAt || '尚未扫描' }}</p>
        <button type="button" :disabled="store.loading" @click="store.refresh()">
          {{ store.loading ? '扫描中…' : '刷新' }}
        </button>
      </div>
    </header>

    <p v-if="store.error" class="err">{{ store.error }}</p>
    <p v-else-if="degraded" class="note">Claude Code 日志为降级质量：同一请求取最大值，输入/输出可能偏低。</p>

    <template v-if="payload">
      <KpiRow :all="payload.all" />
      <ShareBars v-if="payload.by_source.length" :rows="payload.by_source" />

      <div class="split">
        <SliceTable title="按工具" :rows="payload.by_source" :show-turns="true" />
        <SliceTable title="按厂家" :rows="payload.by_vendor" />
      </div>

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
