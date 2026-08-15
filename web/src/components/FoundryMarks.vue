<script setup lang="ts">
import type { CalendarStats } from '../types'

defineProps<{ stats: CalendarStats }>()
</script>

<template>
  <aside class="marks" aria-label="峰值与连烧">
    <div class="stamp">
      <span class="stamp-k">峰值</span>
      <strong class="stamp-v">{{ stats.peak_total_m }}</strong>
      <span class="stamp-s">{{ stats.peak_date || '—' }}</span>
    </div>
    <div class="stamp">
      <span class="stamp-k">当前连烧</span>
      <strong class="stamp-v">{{ stats.current_streak }}<em>天</em></strong>
    </div>
    <div class="stamp">
      <span class="stamp-k">最长连烧</span>
      <strong class="stamp-v">{{ stats.longest_streak }}<em>天</em></strong>
    </div>
  </aside>
</template>
