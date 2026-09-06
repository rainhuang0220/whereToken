<script setup lang="ts">
import { ref } from 'vue'
import KilnKid from '../components/KilnKid.vue'
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
        <p class="status-line">Privacy</p>
        <div class="rail-actions">
          <router-link class="lever" to="/app">Dashboard</router-link>
          <router-link class="lever" to="/settings/devices">设备</router-link>
        </div>
      </div>
    </header>

    <section class="why">
      <p class="cold-kicker">Synced</p>
      <ul>
        <li v-for="item in syncedItems" :key="item">{{ item }}</li>
      </ul>
      <p class="cold-kicker">Never synced</p>
      <ul>
        <li v-for="item in neverSyncedItems" :key="item">{{ item }}</li>
      </ul>
    </section>
    <section class="why">
      <p class="cold-kicker">Account controls</p>
      <p v-if="msg" role="status">{{ msg }}</p>
      <div class="damper">
        <button
          type="button"
          class="lever"
          :disabled="busy"
          @click="del('/api/v1/account/usage', '已删除云端用量，账号仍保留')"
        >
          删除已同步数据
        </button>
        <button
          type="button"
          class="lever"
          :disabled="busy"
          @click="del('/api/v1/account', '账号已删除')"
        >
          删除账号
        </button>
      </div>
    </section>
  </div>
</template>
