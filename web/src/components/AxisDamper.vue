<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { damperIndex, damperStep, damperTabs } from '../axisDamper'
import type { AxisSel, SliceView } from '../types'

const props = defineProps<{
  modelValue: AxisSel
  sources: SliceView[]
  vendors: SliceView[]
}>()

const emit = defineEmits<{
  'update:modelValue': [AxisSel]
}>()

const list = ref<HTMLElement | null>(null)
const tabs = computed(() => damperTabs(props.sources, props.vendors))
const current = computed(() => damperIndex(tabs.value, props.modelValue))

function select(i: number) {
  const tab = tabs.value[i]
  if (!tab) return
  emit('update:modelValue', { kind: tab.kind, id: tab.id })
}

function onKey(e: KeyboardEvent) {
  const next = damperStep(current.value, e.key, tabs.value.length)
  if (next === current.value) return
  e.preventDefault()
  select(next)
  void nextTick(() => {
    const el = list.value?.querySelectorAll<HTMLElement>('[role="tab"]')[next]
    el?.focus()
  })
}
</script>

<template>
  <div ref="list" class="damper" role="tablist" aria-label="窑墙轴" @keydown="onKey">
    <template v-for="(tab, i) in tabs" :key="tab.kind + tab.id">
      <span
        v-if="tab.kind === 'source' && tabs[i - 1]?.kind !== 'source'"
        class="damper-rule"
        aria-hidden="true"
      >工具</span>
      <span
        v-if="tab.kind === 'vendor' && tabs[i - 1]?.kind !== 'vendor'"
        class="damper-rule"
        aria-hidden="true"
      >厂家</span>
      <button
        type="button"
        role="tab"
        :aria-selected="i === current"
        :tabindex="i === current ? 0 : -1"
        :class="{ on: i === current }"
        @click="select(i)"
      >
        {{ tab.label }}
      </button>
    </template>
  </div>
</template>

