<script setup lang="ts">
import { computed } from 'vue'
import { monthLabels, type Cell } from '../grid'

const props = defineProps<{
  cells: Cell[]
  peakDate: string
}>()

const weeks = computed(() => (props.cells.at(-1)?.weekIndex ?? 0) + 1)
const months = computed(() => monthLabels(props.cells))
const wdays = ['一', '', '三', '', '五', '', '']

function tip(cell: Cell): string {
  if (cell.kind === 'future') return `${cell.date} · 未到`
  if (cell.kind === 'empty' || !cell.day) return `${cell.date} · 0.00 M`
  const d = cell.day
  return `${d.date} · ${d.total_m}（未命中 ${d.miss_m} · 缓存读 ${d.cache_read_m} · 缓存写 ${d.cache_create_m} · 输出 ${d.output_m}）`
}
</script>

<template>
  <div class="kiln">
    <div class="months" :style="{ gridTemplateColumns: `repeat(${weeks}, var(--cell))` }">
      <span
        v-for="m in months"
        :key="m.label + m.weekIndex"
        :style="{ gridColumn: m.weekIndex + 1 }"
      >{{ m.label }}</span>
    </div>
    <div class="wall-wrap">
      <div class="wdays" aria-hidden="true">
        <span v-for="(w, i) in wdays" :key="i">{{ w }}</span>
      </div>
      <div
        class="wall"
        role="img"
        :aria-label="`过去 ${weeks} 周的 token 消耗窑墙`"
      >
        <button
          v-for="c in cells"
          :key="c.date"
          type="button"
          class="brick"
          :class="{ peak: c.date === peakDate && c.kind === 'lit' }"
          :data-kind="c.kind"
          :data-level="c.level"
          :style="{ '--w': String(c.weekIndex) }"
          :title="tip(c)"
        />
      </div>
    </div>
    <p class="legend">
      冷
      <i data-level="0" />
      <i data-level="1" />
      <i data-level="2" />
      <i data-level="3" />
      <i data-level="4" />
      白热
    </p>
  </div>
</template>
