<script setup lang="ts">
import { computed, ref } from 'vue'
import { brickAriaLabel, brickCaption, type Cell } from '../grid'

const props = defineProps<{
  cells: Cell[]
  peakDate: string
}>()

const weeks = computed(() => (props.cells.at(-1)?.weekIndex ?? 0) + 1)
const hover = ref<Cell | null>(null)
const tipX = ref(0)
const tipY = ref(0)

function enter(cell: Cell, e: PointerEvent) {
  hover.value = cell
  tipX.value = e.clientX
  tipY.value = e.clientY
}

function focusBrick(cell: Cell, e: FocusEvent) {
  hover.value = cell
  const el = e.target as HTMLElement
  const box = el.getBoundingClientRect()
  tipX.value = box.left
  tipY.value = box.bottom
}

function move(e: PointerEvent) {
  if (!hover.value) return
  tipX.value = e.clientX
  tipY.value = e.clientY
}

function leave() {
  hover.value = null
}

const cap = computed(() => (hover.value ? brickCaption(hover.value) : null))
</script>

<template>
  <div class="kiln">
    <div
      class="wall"
      role="img"
      :aria-label="`过去 ${weeks} 周的 token 强度`"
      @pointerleave="leave"
      @pointermove="move"
    >
      <button
        v-for="c in cells"
        :key="c.date"
        type="button"
        class="brick"
        :class="{ peak: c.date === peakDate && c.kind === 'lit' }"
        :data-kind="c.kind"
        :data-level="String(c.level)"
        :aria-label="brickAriaLabel(c)"
        @pointerenter="enter(c, $event)"
        @focus="focusBrick(c, $event)"
        @blur="leave"
      />
    </div>
    <p
      v-if="cap"
      class="kiln-float"
      role="status"
      style="pointer-events: none"
      :style="{ left: tipX + 14 + 'px', top: tipY + 16 + 'px' }"
    >
      <span>{{ cap.date }}</span>
      <span>{{ cap.amount }}</span>
    </p>
    <p class="legend">
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
