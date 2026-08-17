<script setup lang="ts">
import type { KilnKidPose } from '../kilnKid'

withDefaults(
  defineProps<{
    pose?: KilnKidPose
    flap?: 0 | 1
    size?: 'sm' | 'md'
  }>(),
  { pose: 'grin', flap: 0, size: 'md' },
)
</script>

<template>
  <svg
    class="kiln-kid"
    :class="[`is-${size}`, `pose-${pose}`, `flap-${flap}`]"
    :data-pose="pose"
    :data-flap="flap"
    viewBox="0 0 72 86"
    role="img"
    aria-hidden="true"
  >
    <title>窑崽</title>
    <g class="kid-glow" aria-hidden="true">
      <ellipse cx="36" cy="80" rx="18" ry="4.5" />
    </g>
    <g class="kid-body">
      <!-- kiln pot -->
      <path
        class="kid-pot"
        d="M18 40c0-3 4-8 18-8s18 5 18 8v18c0 10-8 16-18 16S18 68 18 58Z"
      />
      <path class="kid-rim" d="M20 40h32" />
      <!-- steam tuft -->
      <g class="kid-tuft">
        <path d="M26 22c3-10 8-12 10-2" />
        <path d="M36 20c3-11 8-12 10 1" />
      </g>
      <!-- head -->
      <circle class="kid-face" cx="36" cy="32" r="16.5" />
      <circle class="kid-blush l" cx="24.5" cy="35" r="3.2" />
      <circle class="kid-blush r" cx="47.5" cy="35" r="3.2" />
      <g class="kid-eyes">
        <circle class="kid-eye l" cx="29" cy="30" r="2.3" />
        <circle class="kid-eye r" cx="43" cy="30" r="2.3" />
      </g>
      <path class="kid-mouth" d="M30 38q6 6 12 0" />
      <!-- arms -->
      <g class="kid-arms">
        <path class="kid-arm l" d="M21 46c-7 3-9 10-4 14" />
        <path class="kid-arm r" d="M51 46c7 3 9 10 4 14" />
      </g>
      <!-- abacus -->
      <g class="kid-abacus" aria-hidden="true">
        <rect x="23" y="54" width="26" height="9" rx="1.4" />
        <line x1="26" y1="54" x2="26" y2="63" />
        <line x1="36" y1="54" x2="36" y2="63" />
        <line x1="46" y1="54" x2="46" y2="63" />
        <circle cx="26" cy="57" r="1.5" />
        <circle cx="36" cy="59" r="1.5" />
        <circle cx="46" cy="57" r="1.5" />
      </g>
      <circle class="kid-coal" cx="58" cy="24" r="3.4" />
    </g>
  </svg>
</template>
