<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import gsap from 'gsap'
import { chargeAmount, type ScanProgress } from '../firing'
import { tweenCharge, tweenVeil } from '../firingMotion'
import { prefersReducedMotion } from '../themes/galleryMotion'

const props = defineProps<{
  progress: ScanProgress | null
}>()

const root = ref<HTMLElement | null>(null)
const bar = ref<HTMLElement | null>(null)
let ctx: gsap.Context | undefined

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
})

watch(
  () => props.progress,
  () => paintCharge(),
  { deep: true },
)

onUnmounted(() => {
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
      <p class="firing-kicker">煅烧</p>
      <p class="firing-step">{{ progress?.label || '正在读本机账本…' }}</p>
      <div class="firing-track" aria-hidden="true">
        <i ref="bar" class="firing-charge" />
      </div>
    </div>
  </div>
</template>
