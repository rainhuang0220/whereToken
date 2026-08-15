<script setup lang="ts">
import { computed, ref } from 'vue'
import { monthLabels, type Cell } from '../grid'

const props = defineProps<{
  cells: Cell[]
  peakDate: string
}>()

const weeks = computed(() => (props.cells.at(-1)?.weekIndex ?? 0) + 1)
const months = computed(() => monthLabels(props.cells))
const wdays = ['一', '', '三', '', '五', '', '']
const hover = ref<Cell | null>(null)

function tip(cell: Cell): string {
  if (cell.kind === 'future') return `${cell.date} · 未到`
  if (cell.kind === 'empty' || !cell.day) return `${cell.date} · 0.00 M`
  const d = cell.day
  return `${d.date} · ${d.total_m}（未命中 ${d.miss_m} · 缓存读 ${d.cache_read_m} · 缓存写 ${d.cache_create_m} · 输出 ${d.output_m}）`
}

function enter(cell: Cell) {
  hover.value = cell
}

function leave() {
  hover.value = null
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
        @pointerleave="leave"
      >
        <span
          v-for="c in cells"
          :key="c.date"
          class="brick"
          :class="{ peak: c.date === peakDate && c.kind === 'lit' }"
          :data-kind="c.kind"
          :data-level="String(c.level)"
          :title="tip(c)"
          @pointerenter="enter(c)"
        />
      </div>
    </div>
    <p v-if="hover" class="kiln-tip" role="status">{{ tip(hover) }}</p>
    <p v-else class="legend">
      冷
      <i data-kind="empty" />
      <i data-kind="lit" data-level="1" />
      <i data-kind="lit" data-level="2" />
      <i data-kind="lit" data-level="3" />
      <i data-kind="lit" data-level="4" />
      白热
    </p>
  </div>
</template>
