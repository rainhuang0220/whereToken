<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { fetchSummary } from '../api'
import CopyCommand from '../components/CopyCommand.vue'
import HostedShell from '../components/HostedShell.vue'
import { csrfHeaders } from '../csrf'
import type { SummaryPayload } from '../types'

const rows = ref<NonNullable<NonNullable<SummaryPayload['hosted']>['devices']>>([])
const error = ref('')

onMounted(async () => {
  try {
    const p = await fetchSummary('all')
    rows.value = p.hosted?.devices ?? []
  } catch (e) {
    error.value = e instanceof Error ? e.message : '无法加载设备'
  }
})

async function revoke(id: string) {
  const res = await fetch(`/api/v1/devices/${encodeURIComponent(id)}/revoke`, {
    method: 'POST',
    credentials: 'same-origin',
    headers: csrfHeaders(),
  })
  if (!res.ok) {
    error.value = '撤销失败'
    return
  }
  rows.value = rows.value.filter((d) => d.id !== id)
}

function when(iso?: string) {
  if (!iso || iso.startsWith('0001')) return '—'
  return iso.replace('T', ' ').replace('Z', ' UTC')
}
</script>

<template>
  <HostedShell>
    <main class="hosted-card-page">
      <h1>Devices</h1>
      <p v-if="error" class="err" role="alert">{{ error }}</p>
      <ul v-if="rows.length" class="hosted-device-list">
        <li v-for="d in rows" :key="d.id" class="hosted-card">
          <strong>{{ d.label || 'unnamed' }}</strong>
          <p class="muted">{{ [d.os, d.arch, d.client_version].filter(Boolean).join(' · ') }}</p>
          <p class="status-line">Last seen {{ when(d.last_seen) }}</p>
          <p class="status-line">Last sync {{ when(d.last_sync) }}</p>
          <button type="button" class="lever" @click="revoke(d.id)">Revoke</button>
        </li>
      </ul>
      <section v-else class="hosted-card">
        <p class="cold-kicker">Empty</p>
        <p>还没有已连接的设备。</p>
        <CopyCommand command="wheretoken login" />
      </section>
    </main>
  </HostedShell>
</template>
