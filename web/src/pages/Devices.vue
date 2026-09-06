<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { fetchSummary } from '../api'
import CopyCommand from '../components/CopyCommand.vue'
import KilnKid from '../components/KilnKid.vue'
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
        <p class="status-line">Devices</p>
        <div class="rail-actions">
          <router-link class="lever" to="/app">Dashboard</router-link>
          <router-link class="lever" to="/settings/privacy">隐私</router-link>
        </div>
      </div>
    </header>
    <p v-if="error" class="err" role="alert">{{ error }}</p>
    <section v-if="!rows.length" class="cold-kiln">
      <div>
        <p class="cold-kicker">尚未连接设备</p>
        <CopyCommand command="wheretoken login" />
      </div>
    </section>
    <table v-else>
      <thead>
        <tr>
          <th class="name">设备</th>
          <th class="name">系统</th>
          <th class="name">Last seen</th>
          <th class="name">Last sync</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="d in rows" :key="d.id">
          <td class="name">{{ d.label || 'unnamed' }}</td>
          <td class="name">{{ [d.os, d.arch, d.client_version].filter(Boolean).join(' · ') }}</td>
          <td class="name">{{ when(d.last_seen) }}</td>
          <td class="name">{{ when(d.last_sync) }}</td>
          <td><button type="button" class="lever" @click="revoke(d.id)">Revoke</button></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
