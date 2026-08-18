<script setup lang="ts">
import { computed } from 'vue'
import { kilnEyePhase } from '../kilnKid'

const props = withDefaults(
  defineProps<{
    phase?: number
    size?: 'sm' | 'md'
  }>(),
  { phase: 0, size: 'sm' },
)

const eyes = computed(() => kilnEyePhase(props.phase))
</script>

<template>
  <svg
    class="kiln-kid"
    :class="[`is-${size}`]"
    :data-phase="phase"
    viewBox="0 0 8 8"
    role="img"
    aria-hidden="true"
  >
    <title>窑</title>
    <rect class="kid-brick" x="0" y="0" width="8" height="8" />
    <rect class="kid-eye" :x="eyes.a[0]" :y="eyes.a[1]" width="2" height="2" />
    <rect class="kid-eye" :x="eyes.b[0]" :y="eyes.b[1]" width="2" height="2" />
  </svg>
</template>
