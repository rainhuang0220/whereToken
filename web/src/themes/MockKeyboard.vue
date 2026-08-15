<script setup lang="ts">
import gsap from 'gsap'
import { onMounted, onUnmounted, ref } from 'vue'
import { prefersReducedMotion } from './galleryMotion'
import {
  MAIN_ROWS,
  applyPress,
  applyRelease,
  createKeyboardSession,
  isGap,
  shouldPreventDefault,
  type KeySpec,
} from './mockKeyboard'

const root = ref<HTMLElement | null>(null)
const session = createKeyboardSession()

function node(code: string): HTMLElement | null {
  return root.value?.querySelector(`[data-code="${CSS.escape(code)}"]`) ?? null
}

function reduced() {
  return prefersReducedMotion()
}

function press(code: string) {
  const el = node(code)
  if (!el) return
  applyPress(el, { reduced: reduced() })
}

function release(code: string) {
  const el = node(code)
  if (!el) return
  applyRelease(el, { reduced: reduced() })
}

function onKeyDown(e: KeyboardEvent) {
  if (shouldPreventDefault(e)) e.preventDefault()
  if (session.keydown(e).animate) press(e.code)
}

function onKeyUp(e: KeyboardEvent) {
  if (!session.isPressed(e.code)) return
  session.keyup(e.code)
  release(e.code)
}

function onBlur() {
  for (const code of session.releaseAll()) release(code)
}

function onPointerDown(e: PointerEvent, code: string) {
  e.preventDefault()
  e.stopPropagation()
  const t = e.currentTarget
  if (t instanceof HTMLElement) t.setPointerCapture(e.pointerId)
  if (session.keydown({ code, repeat: false }).animate) press(code)
}

function onPointerUp(e: PointerEvent, code: string) {
  e.stopPropagation()
  if (!session.isPressed(code)) return
  session.keyup(code)
  release(code)
}

function keyStyle(key: KeySpec): Record<string, string> {
  return { '--u': String(key.u) }
}

function gapStyle(n: number): Record<string, string> {
  return { '--u': String(n) }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('keyup', onKeyUp)
  window.addEventListener('blur', onBlur)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('keyup', onKeyUp)
  window.removeEventListener('blur', onBlur)
  session.releaseAll()
  if (root.value) gsap.killTweensOf(root.value.querySelectorAll('.kb-key'))
})
</script>

<template>
  <div ref="root" class="kb" @click.stop>
    <div class="kb-main">
      <div v-for="(row, ri) in MAIN_ROWS" :key="ri" class="kb-row" :class="{ 'is-fn': ri === 0 }">
        <template v-for="(slot, si) in row" :key="si">
          <span v-if="isGap(slot)" class="kb-gap" :style="gapStyle(slot.gap)" />
          <span
            v-else
            class="kb-key"
            :class="{ wide: slot.u >= 4 }"
            :data-code="slot.code"
            :data-fill="slot.fill"
            :style="keyStyle(slot)"
            @pointerdown="onPointerDown($event, slot.code)"
            @pointerup="onPointerUp($event, slot.code)"
            @pointercancel="onPointerUp($event, slot.code)"
          >{{ slot.label }}</span>
        </template>
      </div>
    </div>
  </div>
</template>
