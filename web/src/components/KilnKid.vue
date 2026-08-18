<script setup lang="ts">
import { computed } from 'vue'
import { kilnKidPose, type KilnKidPose } from '../kilnKid'

const props = withDefaults(
  defineProps<{
    pose?: KilnKidPose
    phase?: number
    size?: 'sm' | 'md'
  }>(),
  { size: 'sm' },
)

const pose = computed(() => props.pose ?? kilnKidPose(props.phase ?? 0))
</script>

<template>
  <!-- Claude-style slab: filled block, two vertical eye slots. -->
  <svg
    class="kiln-kid"
    :class="[`is-${size}`, `pose-${pose}`]"
    :data-pose="pose"
    viewBox="0 0 48 28"
    role="img"
    aria-hidden="true"
  >
    <title>窑</title>
    <rect class="kid-body" x="1" y="3" width="46" height="22" rx="4" />
    <rect class="kid-eye" x="15" y="8" width="3.4" height="9" rx="0.8" />
    <rect class="kid-eye r" x="29.6" y="8" width="3.4" height="9" rx="0.8" />
  </svg>
</template>
