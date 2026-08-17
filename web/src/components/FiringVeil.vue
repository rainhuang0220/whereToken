<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import gsap from 'gsap'
import { chargeAmount, type ScanProgress } from '../firing'
import { tweenCharge, tweenVeil } from '../firingMotion'
import { kilnKidMood, kilnKidPose, kilnTipAt, type KilnKidPose } from '../kilnKid'
import { prefersReducedMotion } from '../themes/galleryMotion'
import KilnKid from './KilnKid.vue'

const props = defineProps<{
  progress: ScanProgress | null
}>()

const root = ref<HTMLElement | null>(null)
const bar = ref<HTMLElement | null>(null)
const pose = ref<KilnKidPose>(kilnKidPose(0))
const mood = ref(kilnKidMood(0))
const flap = ref<0 | 1>(0)
const tip = ref(kilnTipAt(0))
let ctx: gsap.Context | undefined
let kidTimer: number | undefined
let kidTick = 0

function reduced() {
  return prefersReducedMotion()
}

function paintCharge() {
  if (!bar.value) return
  tweenCharge(bar.value, chargeAmount(props.progress), reduced())
}

onMounted(() => {
  if (!root.value) return
  ctx = gsap.context(() => {
    if (root.value) tweenVeil(root.value, true, reduced())
    paintCharge()
  }, root.value)
  if (!reduced()) {
    kidTimer = window.setInterval(() => {
      kidTick += 1
      pose.value = kilnKidPose(Math.floor(kidTick / 2))
      mood.value = kilnKidMood(Math.floor(kidTick / 2))
      flap.value = (kidTick % 2) as 0 | 1
      tip.value = kilnTipAt(Math.floor(kidTick / 22))
    }, 90)
  }
})

watch(
  () => props.progress,
  () => paintCharge(),
  { deep: true },
)

onUnmounted(() => {
  if (kidTimer !== undefined) window.clearInterval(kidTimer)
  ctx?.revert()
})
</script>

<template>
  <div
    ref="root"
    class="firing-veil"
    role="status"
    aria-live="polite"
    aria-busy="true"
  >
    <div class="firing-shade" aria-hidden="true" />
    <div class="firing-copy">
      <div class="firing-mascot">
        <KilnKid :pose="pose" :flap="flap" size="md" />
        <p class="firing-mood">{{ mood }}</p>
      </div>
      <div class="firing-words">
        <p class="firing-kicker">煅烧</p>
        <p class="firing-step">{{ progress?.label || '正在读本机账本…' }}</p>
        <p class="firing-tip">{{ tip }}</p>
        <div class="firing-track" aria-hidden="true">
          <i ref="bar" class="firing-charge" />
        </div>
      </div>
    </div>
  </div>
</template>
