<script setup lang="ts">
import type { SliceView } from '../types'

defineProps<{ all: SliceView }>()
</script>

<template>
  <section class="kpi" aria-label="合计">
    <div class="kpi-cell kpi-lead">
      <span class="kpi-label">合计</span>
      <strong class="kpi-figure">{{ all.total_m }}</strong>
    </div>
    <div class="kpi-cell">
      <span class="kpi-label">命中率</span>
      <strong>{{ all.hit_rate_text }}</strong>
    </div>
    <div class="kpi-cell">
      <span class="kpi-label">未命中</span>
      <strong>{{ all.miss_m }}</strong>
    </div>
    <div class="kpi-cell">
      <span class="kpi-label">输出</span>
      <strong>{{ all.output_m }}</strong>
    </div>
    <div class="kpi-cell">
      <span class="kpi-label">请求</span>
      <strong>{{ all.requests }}</strong>
    </div>
    <div class="kpi-cell">
      <span class="kpi-label">用户回合</span>
      <strong>{{ all.user_turns }}</strong>
    </div>
  </section>
</template>
