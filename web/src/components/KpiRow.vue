<script setup lang="ts">
import { formatCount, hitBand } from '../format'
import type { SliceView } from '../types'

defineProps<{ all: SliceView }>()
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
      <span class="read-k">未命中</span>
      <strong>{{ all.miss_m }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">输出</span>
      <strong>{{ all.output_m }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">请求</span>
      <strong>{{ formatCount(all.requests) }}</strong>
    </div>
    <div class="read-cell">
      <span class="read-k">用户回合</span>
      <strong>{{ formatCount(all.user_turns) }}</strong>
    </div>
  </section>
</template>
