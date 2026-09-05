<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { csrfHeaders } from '../csrf'

const route = useRoute()
const router = useRouter()
const code = String(route.params.code || '')
const error = ref('')
const meta = ref<{ os?: string; arch?: string; label?: string; client_version?: string } | null>(null)

onMounted(async () => {
  const me = await fetch('/api/v1/session', { credentials: 'same-origin' })
  if (me.status === 401) {
    window.location.assign(`/api/v1/auth/github?next=${encodeURIComponent('/pair/' + code)}`)
    return
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
    await router.push('/app')
  } else {
    await router.push('/login')
  }
}
</script>

<template>
  <main class="page">
    <h1>连接新设备</h1>
    <p>{{ meta?.label || 'whereToken CLI' }}</p>
    <p class="muted">{{ [meta?.os, meta?.arch, meta?.client_version].filter(Boolean).join(' · ') || code }}</p>
    <p v-if="error">{{ error }}</p>
    <button type="button" class="btn" @click="decide(true)">连接设备</button>
    <button type="button" class="btn" @click="decide(false)">拒绝</button>
  </main>
</template>
