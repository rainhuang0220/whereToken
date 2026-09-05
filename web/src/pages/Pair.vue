<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import HostedShell from '../components/HostedShell.vue'
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

const subtitle = () =>
  [meta.value?.os, meta.value?.arch, meta.value?.client_version].filter(Boolean).join(' · ')
</script>

<template>
  <HostedShell>
    <main class="hosted-card-page">
      <section v-if="done" class="hosted-card">
        <p class="cold-kicker">Pair</p>
        <h1>{{ pairSuccessTitle }}</h1>
        <p class="hosted-lede">{{ pairSuccessBody }}</p>
        <router-link class="lever primary" to="/app">Open Dashboard</router-link>
      </section>
      <section v-else class="hosted-card">
        <p class="cold-kicker">Pair</p>
        <h1>{{ pairTitle }}</h1>
        <p class="hosted-device">{{ meta?.label || 'whereToken CLI' }}</p>
        <p class="muted">{{ subtitle() || code }}</p>
        <p class="hosted-lede">这台设备将可以把聚合后的用量统计同步到你的账号。本地原文不会上传。</p>
        <p v-if="error" class="err" role="alert">{{ error }}</p>
        <div class="hosted-cta-row">
          <button type="button" class="lever primary" @click="decide(true)">Connect device</button>
          <button type="button" class="lever" @click="decide(false)">Cancel</button>
        </div>
      </section>
    </main>
  </HostedShell>
</template>
