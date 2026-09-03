<script setup lang="ts">
import { computed, ref } from 'vue'
import EstimateModal from './EstimateModal.vue'
import { costHonestyNote, formatCost2, formatCount, hitBand, tokenCell } from '../format'
import type { ModelView, SliceView, UsagePortrait } from '../types'

const props = defineProps<{
  all: SliceView
  maxStreak?: number
  currentStreak?: number
  compareText?: string
  todayTotalM?: string
  peakTotalM?: string
  peakDate?: string
  portrait?: UsagePortrait
  byModel?: ModelView[]
  byVendor?: SliceView[]
}>()

// A scan that found nothing is "—", never a fake 0.00 M.
const noData = computed(
  () => !props.all.quality && !props.all.total && !props.all.requests && !props.all.user_turns,
)
const totalText = computed(() =>
  noData.value ? '—' : tokenCell(props.all.total_m, props.all.quality, props.all),
)
const todayText = computed(() => (noData.value ? '—' : props.todayTotalM || '—'))
const peakText = computed(() => (noData.value ? '—' : props.peakTotalM || '—'))
const peakTitle = computed(() => (props.peakDate ? `单日最高 ${props.peakDate}` : undefined))
const estimateText = computed(() => formatCost2(props.all.cost_usd) || '—')
const hasModels = computed(() => (props.byModel?.length ?? 0) > 0)
const estOpen = ref(false)
const honesty = computed(() => costHonestyNote(props.all))

const portraitPrimary = computed(() => {
  const p = props.portrait
  if (!p || p.state === 'none') return '—'
  if (p.state === 'insufficient') return '数据不足'
  return p.primary || '—'
})
const portraitTags = computed(() =>
  props.portrait?.state === 'ok' ? (props.portrait.tags ?? []).filter(Boolean).join(' · ') : '',
)
const portraitTitle = computed(() =>
  props.portrait?.state === 'ok' && props.portrait.detail ? props.portrait.detail : undefined,
)
</script>

<template>
  <section class="readout" aria-label="总用量">
    <div class="read-lead">
      <span class="read-k">总用量</span>
      <strong>{{ totalText }}</strong>
    </div>
    <div class="read-cell" :data-hit="hitBand(all.hit_rate_text)">
      <span class="read-k">命中率</span>
      <strong>{{ all.hit_rate_text }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">最长连烧</span>
      <strong>{{ formatCount(maxStreak || 0) }} 天</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">当日用量</span>
      <strong>{{ todayText }}</strong>
    </div>
    <button
      type="button"
      class="read-cell read-btn"
      :disabled="!hasModels"
      title="当前周期总估价 · API 等价估算，非实际账单"
      @click="estOpen = true"
    >
      <span class="read-k">估价</span>
      <strong>{{ estimateText }}</strong>
      <span class="read-sub">价目与来源见 wheretoken pricing</span>
    </button>
    <div class="read-cell">
      <span class="read-k">当前连烧</span>
      <strong>{{ formatCount(currentStreak || 0) }} 天</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">请求</span>
      <strong>{{ formatCount(all.requests) }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">用户回合</span>
      <strong>{{ formatCount(all.user_turns) }}</strong>
    </div>
    <div class="read-cell" :title="peakTitle">
      <span class="read-k">单日最高</span>
      <strong>{{ peakText }}</strong>
    </div>
    <div class="read-cell" :title="portraitTitle">
      <span class="read-k">用户画像</span>
      <strong>{{ portraitPrimary }}</strong>
      <span v-if="portraitTags" class="read-sub">{{ portraitTags }}</span>
    </div>
    <p v-if="compareText" class="read-compare">{{ compareText }}</p>
    <p v-if="honesty" class="read-compare">{{ honesty }}</p>
  </section>
  <EstimateModal
    v-if="estOpen"
    :models="byModel ?? []"
    :vendors="byVendor ?? []"
    :total="estimateText"
    @close="estOpen = false"
  />
</template>
