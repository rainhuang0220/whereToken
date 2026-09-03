<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { costCaption } from '../format'
import type { ModelView, SliceView } from '../types'

const props = defineProps<{
  models: ModelView[]
  vendors: SliceView[]
  total: string
}>()
const emit = defineEmits<{ close: [] }>()

type Row = {
  key: string
  model: string
  totalM: string
  prices: string
  cost: string
  priced: boolean
}
type Group = { vendor: string; label: string; rows: Row[] }

// Listed card rates only; an unlisted component is omitted, never shown as
// $0.00 (a listed 0, e.g. free cache-write storage, does print).
function unitPrices(m: ModelView): string {
  const parts: string[] = []
  const up = m.unit_prices
  if (up) {
    if (up.miss != null) parts.push(`in $${up.miss.toFixed(2)}`)
    if (up.cache_read != null) parts.push(`缓存读 $${up.cache_read.toFixed(2)}`)
    if (up.cache_create != null) parts.push(`缓存写 $${up.cache_create.toFixed(2)}`)
    if (up.output != null) parts.push(`出 $${up.output.toFixed(2)}`)
  }
  return parts.join(' · ')
}

const groups = computed<Group[]>(() => {
  const labels = new Map(props.vendors.map((v) => [v.id, v.label]))
  const out: Group[] = []
  const byVendor = new Map<string, Group>()
  for (const m of props.models) {
    let g = byVendor.get(m.vendor)
    if (!g) {
      g = { vendor: m.vendor, label: labels.get(m.vendor) || m.vendor || '(未知厂家)', rows: [] }
      byVendor.set(m.vendor, g)
      out.push(g)
    }
    const cost = costCaption(m) || '—'
    g.rows.push({
      key: `${m.vendor}/${m.id || m.label}`,
      model: m.label || m.id || '(未知模型)',
      totalM: m.total_m,
      prices: unitPrices(m) || '—',
      cost,
      priced: cost !== '—',
    })
  }
  return out
})

const hasUnpriced = computed(() => groups.value.some((g) => g.rows.some((r) => !r.priced)))

const veil = ref<HTMLElement>()
onMounted(() => {
  veil.value?.querySelector<HTMLElement>('.est-close')?.focus()
})

function close() {
  emit('close')
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    close()
    return
  }
  if (e.key !== 'Tab' || !veil.value) return
  const items = Array.from(
    veil.value.querySelectorAll<HTMLElement>('button, [href], [tabindex]:not([tabindex="-1"])'),
  ).filter((el) => !el.hasAttribute('disabled'))
  if (!items.length) return
  const first = items[0]
  const last = items[items.length - 1]
  const active = document.activeElement
  if (e.shiftKey && (active === first || !veil.value.contains(active))) {
    e.preventDefault()
    last.focus()
  } else if (!e.shiftKey && (active === last || !veil.value.contains(active))) {
    e.preventDefault()
    first.focus()
  }
}
</script>

<template>
  <div ref="veil" class="est-veil" @click.self="close" @keydown="onKeydown">
    <div class="est-modal" role="dialog" aria-modal="true" aria-label="估价明细">
      <div class="est-head">
        <h2>估价明细</h2>
        <button type="button" class="est-close" aria-label="关闭估价明细" @click="close">×</button>
      </div>
      <p class="est-note">API 等价估算，非实际账单 · 单价 USD / 1M tokens</p>
      <div class="est-body">
        <section v-for="g in groups" :key="g.vendor" class="est-group">
          <h3>{{ g.label }}</h3>
          <table>
            <thead>
              <tr>
                <th class="name">模型</th>
                <th class="num">用量</th>
                <th class="name">单价 /1M</th>
                <th class="num">估价</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in g.rows" :key="row.key">
                <td class="name">{{ row.model }}</td>
                <td class="num">{{ row.totalM }}</td>
                <td class="name est-prices">{{ row.prices }}</td>
                <td class="num">{{ row.cost }}</td>
              </tr>
            </tbody>
          </table>
        </section>
      </div>
      <div class="est-total">
        <span>合计</span>
        <strong>{{ total }}</strong>
      </div>
      <p v-if="hasUnpriced" class="est-note">部分用量无价：— 表示该模型不在公开价目中，相应用量未计入合计。</p>
    </div>
  </div>
</template>
