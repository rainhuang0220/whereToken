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
  <svg
    class="kiln-kid"
    :class="[`is-${size}`, `pose-${pose}`]"
    :data-pose="pose"
    viewBox="0 0 32 36"
    role="img"
    aria-hidden="true"
  >
    <title>窑崽</title>
    <!-- brick body -->
    <rect class="kid-brick" x="4" y="8" width="24" height="24" rx="3" />
    <rect class="kid-lid" x="7" y="4" width="18" height="6" rx="2" />
    <!-- hole eyes (void) -->
    <rect class="kid-eye l" x="9" y="16" width="5" height="5" rx="1" />
    <rect class="kid-eye r" x="18" y="16" width="5" height="5" rx="1" />
    <rect class="kid-mouth" x="13" y="24" width="6" height="2" rx="1" />
  </svg>
</template>
