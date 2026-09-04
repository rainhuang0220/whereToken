<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { fetchSummary } from '../api'
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
  })
  if (!res.ok) {
    error.value = '撤销失败'
    return
  }
  rows.value = rows.value.filter((d) => d.id !== id)
}
</script>

<template>
  <main class="page">
    <h1>设备</h1>
    <p v-if="error">{{ error }}</p>
    <ul>
      <li v-for="d in rows" :key="d.id">
        <strong>{{ d.label }}</strong>
        {{ d.os }} {{ d.arch }}
        <span>Last seen {{ d.last_seen || '—' }}</span>
        <span>Last sync {{ d.last_sync || '—' }}</span>
        <button type="button" @click="revoke(d.id)">Revoke</button>
      </li>
    </ul>
    <p v-if="!rows.length">还没有已连接的设备。在本机运行 <code>wheretoken login</code>。</p>
  </main>
</template>
