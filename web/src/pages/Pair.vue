<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import KilnKid from '../components/KilnKid.vue'
import { csrfHeaders } from '../csrf'
import { pairSuccessBody, pairSuccessTitle, pairTitle } from '../hosted/copy'

const route = useRoute()
const code = String(route.params.code || '')
const error = ref('')
const done = ref(false)
const meta = ref<{ os?: string; arch?: string; label?: string; client_version?: string } | null>(null)

onMounted(async () => {
  const me = await fetch('/api/v1/session', { credentials: 'same-origin' })
  if (me.status === 401) {
    window.location.assign(`/api/v1/auth/github?next=${encodeURIComponent('/pair/' + code)}`)
    return
  }
  const res = await fetch(`/api/v1/pair/challenge?code=${encodeURIComponent(code)}`, {
    credentials: 'same-origin',
  })
  if (res.ok) {
    meta.value = (await res.json()) as typeof meta.value
  }
})

async function decide(accept: boolean) {
  error.value = ''
  const res = await fetch('/api/v1/pair/confirm', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
    body: JSON.stringify({ display_code: code, accept }),
  })
  if (!res.ok) {
    error.value = '无法完成配对'
    return
  }
  if (accept) {
    done.value = true
    return
  }
  window.location.assign('/app')
}
</script>

<template>
  <div class="forge">
    <header class="rail">
      <div class="rail-brand">
        <KilnKid pose="grin" size="sm" />
        <div class="rail-name">
          <h1>whereToken</h1>
          <p class="whisper">本机 token 窑</p>
        </div>
      </div>
      <div class="rail-meta">
        <p class="status-line">Pair</p>
        <div class="rail-actions">
          <router-link class="lever" to="/app">Dashboard</router-link>
        </div>
      </div>
    </header>

    <section v-if="done" class="cold-kiln">
      <KilnKid pose="grin" size="md" />
      <div>
        <p class="cold-kicker">{{ pairSuccessTitle }}</p>
        <p class="cold-copy">{{ pairSuccessBody }}</p>
        <router-link class="lever primary" to="/app">Open Dashboard</router-link>
      </div>
    </section>
    <section v-else class="cold-kiln">
      <KilnKid pose="blink" size="md" />
      <div>
        <p class="cold-kicker">{{ pairTitle }}</p>
        <p class="cold-copy">{{ meta?.label || 'whereToken CLI' }}</p>
        <p class="note">{{ [meta?.os, meta?.arch, meta?.client_version].filter(Boolean).join(' · ') || code }}</p>
        <p class="cold-copy">这台设备将同步聚合用量。prompts、代码和路径不会上传。</p>
        <p v-if="error" class="err" role="alert">{{ error }}</p>
        <div class="damper">
          <button type="button" class="lever primary" @click="decide(true)">Connect device</button>
          <button type="button" class="lever" @click="decide(false)">Cancel</button>
        </div>
      </div>
    </section>
  </div>
</template>
