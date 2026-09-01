<script setup lang="ts">
import { computed } from 'vue'
import { costHonestyNote, costKPI, formatCount, hitBand, tokenCell } from '../format'
import type { SliceView, UsageEvaluation } from '../types'

const props = defineProps<{
  all: SliceView
  maxStreak?: number
  currentStreak?: number
  compareText?: string
  evaluation?: UsageEvaluation
}>()

// A scan that found nothing is "—", never a fake 0.00 M.
const totalText = computed(() =>
  !props.all.quality && !props.all.total && !props.all.requests && !props.all.user_turns
    ? '—'
    : tokenCell(props.all.total_m, props.all.quality, props.all),
)
const honesty = computed(() => costHonestyNote(props.all))
const evalSummary = computed(() => props.evaluation?.summary || '—')
const evalReason = computed(() => props.evaluation?.reason || '')
</script>

<template>
  <section class="readout" aria-label="合计">
    <div class="read-lead">
      <span class="read-k">合计</span>
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
    <div
      class="read-cell"
      title="Estimated API-equivalent cost based on public model pricing. This is not your actual subscription bill. Run `wheretoken pricing` to see the rate card and where each price comes from."
    >
      <span class="read-k">估价</span>
      <strong>{{ costKPI(all) }}</strong>
      <span class="read-sub">价目与来源见 wheretoken pricing</span>
    </div>
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
    <div class="read-cell" :title="evalReason || undefined">
      <span class="read-k">用量评价</span>
      <strong>{{ evalSummary }}</strong>
      <span v-if="evalReason" class="read-sub">{{ evalReason }}</span>
    </div>
    <p v-if="compareText" class="read-compare">{{ compareText }}</p>
    <p v-if="honesty" class="read-compare">{{ honesty }}</p>
  </section>
</template>
