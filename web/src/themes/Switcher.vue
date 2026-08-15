<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { STORAGE_KEY, applyTheme, resolveThemeId, themes, type ThemeId } from './index'

const current = ref<ThemeId>('kiln')

onMounted(() => {
  let stored: string | null = null
  try {
    stored = localStorage.getItem(STORAGE_KEY)
  } catch {
    stored = null
  }
  current.value = resolveThemeId(stored)
  applyTheme(current.value)
})

function pick(id: ThemeId) {
  current.value = id
  applyTheme(id)
}
</script>

<template>
  <div class="packs" role="radiogroup" aria-label="釉色">
    <button
      v-for="t in themes"
      :key="t.id"
      type="button"
      role="radio"
      :aria-checked="current === t.id"
      :aria-label="t.name"
      :class="{ on: current === t.id }"
      :title="t.name"
      @click="pick(t.id)"
    >
      {{ t.mark }}
    </button>
  </div>
</template>
