<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { STORAGE_KEY, applyTheme, resolveThemeId, themes, type ThemeId, type ThemePack } from '../themes'

const router = useRouter()
const committed = ref<ThemeId>('kiln')
const chosen = ref<ThemeId>('kiln')
let applied = false

onMounted(() => {
  let stored: string | null = null
  try {
    stored = localStorage.getItem(STORAGE_KEY)
  } catch {
    stored = null
  }
  committed.value = resolveThemeId(stored)
  chosen.value = committed.value
})

onBeforeUnmount(() => {
  if (!applied) applyTheme(committed.value, { persist: false })
})

function preview(id: ThemeId) {
  chosen.value = id
  applyTheme(id, { persist: false })
}

function commit() {
  applied = true
  applyTheme(chosen.value)
  void router.push('/')
}

function slabVars(t: ThemePack): Record<string, string> {
  return {
    background: t.tokens.void,
    color: t.tokens.bone,
    borderColor: t.tokens.copper,
    '--clay': t.tokens.clay,
    '--mortar': t.tokens.mortar,
    '--ember-1': t.tokens['ember-1'],
    '--ember-2': t.tokens['ember-2'],
    '--ember-3': t.tokens['ember-3'],
    '--ember-4': t.tokens['ember-4'],
    '--bone': t.tokens.bone,
    '--ash': t.tokens.ash,
    '--copper': t.tokens.copper,
    '--void': t.tokens.void,
  }
}
</script>

<template>
  <div class="glaze-hall">
    <header class="rail">
      <h1>釉</h1>
      <div class="rail-meta">
        <p class="when">点一块试釉，应用后回窑墙</p>
        <div class="rail-actions">
          <router-link class="lever" to="/">返回</router-link>
          <button type="button" class="lever primary" @click="commit">应用</button>
        </div>
      </div>
    </header>

    <div class="glaze-shelf">
      <button
        v-for="t in themes"
        :key="t.id"
        type="button"
        class="glaze-slab"
        :class="{ on: chosen === t.id }"
        :style="slabVars(t)"
        :aria-pressed="chosen === t.id"
        :aria-label="t.name"
        @click="preview(t.id)"
      >
        <span class="glaze-mark">{{ t.mark }}</span>
        <span class="glaze-name">{{ t.name }}</span>
        <span class="glaze-ramp" aria-hidden="true">
          <i data-kind="empty" />
          <i data-kind="lit" data-level="1" />
          <i data-kind="lit" data-level="2" />
          <i data-kind="lit" data-level="3" />
          <i data-kind="lit" data-level="4" />
        </span>
        <span class="glaze-chip">空砖 → 白热</span>
      </button>
    </div>
  </div>
</template>
