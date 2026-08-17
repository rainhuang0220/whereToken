<script setup lang="ts">
import type { KilnKidPose } from '../kilnKid'

withDefaults(
  defineProps<{
    pose?: KilnKidPose
    size?: 'sm' | 'md'
  }>(),
  { pose: 'grin', size: 'md' },
)
</script>

<template>
  <svg
    class="kiln-kid"
    :class="[`is-${size}`, `pose-${pose}`]"
    :data-pose="pose"
    viewBox="0 0 64 78"
    role="img"
    aria-hidden="true"
  >
    <title>窑崽</title>
    <g class="kid-glow" aria-hidden="true">
      <ellipse cx="32" cy="70" rx="16" ry="4" />
    </g>
    <g class="kid-body">
      <path
        class="kid-pot"
        d="M16 36c0-2 3-6 16-6s16 4 16 6v16c0 8-7 14-16 14S16 60 16 52Z"
      />
      <path class="kid-rim" d="M18 36h28" />
      <g class="kid-tuft">
        <path d="M24 18c2-8 6-10 8-2" />
        <path d="M32 16c2-8 6-10 8 0" />
      </g>
      <circle class="kid-face" cx="32" cy="30" r="15" />
      <g class="kid-eyes">
        <circle class="kid-eye l" cx="26" cy="28" r="2.1" />
        <circle class="kid-eye r" cx="38" cy="28" r="2.1" />
      </g>
      <path class="kid-mouth" d="M26 35q6 6 12 0" />
      <g class="kid-arms">
        <path class="kid-arm l" d="M18 42c-6 2-8 8-4 12" />
        <path class="kid-arm r" d="M46 42c6 2 8 8 4 12" />
      </g>
      <g class="kid-abacus" aria-hidden="true">
        <rect x="20" y="48" width="24" height="8" rx="1" />
        <circle cx="26" cy="52" r="1.6" />
        <circle cx="32" cy="52" r="1.6" />
        <circle cx="38" cy="52" r="1.6" />
      </g>
      <circle class="kid-coal" cx="50" cy="22" r="3" />
    </g>
  </svg>
</template>
