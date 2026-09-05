<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{ command: string }>()
const copied = ref(false)

async function copy() {
  try {
    await navigator.clipboard.writeText(props.command)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1600)
  } catch {
    copied.value = false
  }
}
</script>

<template>
  <div class="copy-cmd">
    <code>{{ command }}</code>
    <button type="button" class="lever" :aria-label="'复制 ' + command" @click="copy">
      {{ copied ? '已复制' : '复制' }}
    </button>
  </div>
</template>
