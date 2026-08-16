<script setup lang="ts">
import type { AxisSel, SliceView } from '../types'

defineProps<{
  modelValue: AxisSel
  sources: SliceView[]
  vendors: SliceView[]
}>()

const emit = defineEmits<{
  'update:modelValue': [AxisSel]
}>()
</script>

<template>
  <div class="damper" role="tablist" aria-label="窑墙轴">
    <button
      type="button"
      role="tab"
      :aria-selected="modelValue.kind === 'all'"
      :class="{ on: modelValue.kind === 'all' }"
      @click="emit('update:modelValue', { kind: 'all', id: 'all' })"
    >
      合计
    </button>
    <span class="damper-rule" aria-hidden="true">工具</span>
    <button
      v-for="row in sources"
      :key="'s' + row.id"
      type="button"
      role="tab"
      :aria-selected="modelValue.kind === 'source' && modelValue.id === row.id"
      :class="{ on: modelValue.kind === 'source' && modelValue.id === row.id }"
      @click="emit('update:modelValue', { kind: 'source', id: row.id })"
    >
      {{ row.label }}
    </button>
    <span class="damper-rule" aria-hidden="true">厂家</span>
    <button
      v-for="row in vendors"
      :key="'v' + row.id"
      type="button"
      role="tab"
      :aria-selected="modelValue.kind === 'vendor' && modelValue.id === row.id"
      :class="{ on: modelValue.kind === 'vendor' && modelValue.id === row.id }"
      @click="emit('update:modelValue', { kind: 'vendor', id: row.id })"
    >
      {{ row.label }}
    </button>
  </div>
</template>
