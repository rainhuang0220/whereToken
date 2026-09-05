<script setup lang="ts">
import { ref } from 'vue'
import HostedShell from '../components/HostedShell.vue'
import { csrfHeaders } from '../csrf'
import { neverSyncedItems, syncedItems } from '../hosted/copy'

const msg = ref('')
const busy = ref(false)

async function del(path: string, okText: string) {
  busy.value = true
  msg.value = ''
  try {
    const res = await fetch(path, {
      method: 'DELETE',
      credentials: 'same-origin',
      headers: csrfHeaders(),
    })
    if (res.status === 401) {
      window.location.assign('/login')
      return
    }
    if (!res.ok) {
      msg.value = '操作失败'
      return
    }
    msg.value = okText
    if (path === '/api/v1/account') {
      window.location.assign('/login')
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <HostedShell>
    <main class="hosted-card-page">
      <h1>Privacy</h1>
      <section class="hosted-card">
        <p class="cold-kicker">Synced</p>
        <ul class="hosted-yes">
          <li v-for="item in syncedItems" :key="item">{{ item }}</li>
        </ul>
      </section>
      <section class="hosted-card">
        <p class="cold-kicker">Never synced</p>
        <ul class="hosted-no">
          <li v-for="item in neverSyncedItems" :key="item">{{ item }}</li>
        </ul>
      </section>
      <section class="hosted-card hosted-danger">
        <p class="cold-kicker">Account controls</p>
        <p v-if="msg" role="status">{{ msg }}</p>
        <p>
          <button
            type="button"
            class="lever"
            :disabled="busy"
            @click="del('/api/v1/account/usage', '已删除云端用量，账号仍保留')"
          >
            删除已同步数据
          </button>
        </p>
        <p>
          <button
            type="button"
            class="lever"
            :disabled="busy"
            @click="del('/api/v1/account', '账号已删除')"
          >
            删除账号
          </button>
        </p>
      </section>
    </main>
  </HostedShell>
</template>
