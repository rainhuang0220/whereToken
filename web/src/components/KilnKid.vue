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
  <!-- Copied from MoonshotAI/kimi-code apps/vscode/resources/kimi-icon.svg; fill/stroke follow our lemon. -->
  <svg
    class="kiln-kid"
    :class="[`is-${size}`, `pose-${pose}`]"
    :data-pose="pose"
    viewBox="0 0 24 24"
    role="img"
    aria-hidden="true"
  >
    <title>窑</title>
    <rect
      class="kid-body"
      x="3"
      y="4.5"
      width="18"
      height="13"
      rx="2.2"
      fill="none"
      stroke="currentColor"
      stroke-width="1.6"
    />
    <rect class="kid-eye" x="9.6" y="8" width="1.4" height="2.6" rx="0.45" fill="currentColor" />
    <rect class="kid-eye r" x="15.6" y="8" width="1.4" height="2.6" rx="0.45" fill="currentColor" />
  </svg>
</template>
