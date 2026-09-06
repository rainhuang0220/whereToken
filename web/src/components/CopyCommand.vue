<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{ command: string }>()
const copied = ref<'ok' | 'fail' | ''>('')

async function copy() {
  try {
    await navigator.clipboard.writeText(props.command)
    copied.value = 'ok'
  } catch {
    copied.value = 'fail'
  }
  window.setTimeout(() => {
    copied.value = ''
  }, 1600)
}
</script>

<template>
  <div class="copy-cmd">
    <code>{{ command }}</code>
    <button
      type="button"
      class="lever"
      :aria-label="'复制 ' + command"
      @click="copy"
    >
      <span aria-live="polite">{{ copied === 'ok' ? '已复制' : copied === 'fail' ? '复制失败' : '复制' }}</span>
    </button>
  </div>
</template>
