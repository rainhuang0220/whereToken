<script setup lang="ts">
import { formatCount, hitBand } from '../format'
import type { SliceView } from '../types'

defineProps<{
  all: SliceView
  todayM?: string
  peakM?: string
  compareText?: string
}>()
</script>

<template>
  <section class="readout" aria-label="合计">
    <div class="read-lead">
      <span class="read-k">合计</span>
      <strong>{{ all.total_m }}</strong>
    </div>
    <div class="read-cell" :data-hit="hitBand(all.hit_rate_text)">
      <span class="read-k">命中率</span>
      <strong>{{ all.hit_rate_text }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">当日用量</span>
      <strong>{{ todayM || '0.00 M' }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">单日最高</span>
      <strong>{{ peakM || '0.00 M' }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">请求</span>
      <strong>{{ formatCount(all.requests) }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">用户回合</span>
      <strong>{{ formatCount(all.user_turns) }}</strong>
    </div>
    <p v-if="compareText" class="read-compare">{{ compareText }}</p>
  </section>
</template>
