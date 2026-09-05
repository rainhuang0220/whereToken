<script setup lang="ts">
import { ref } from 'vue'
import { csrfHeaders } from '../csrf'

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
  <main class="page">
    <h1>同步与隐私</h1>
    <h2>会上传</h2>
    <ul>
      <li>token counts</li>
      <li>models / vendors</li>
      <li>request / turn counts</li>
      <li>dates</li>
      <li>source health</li>
    </ul>
    <h2>不会上传</h2>
    <ul>
      <li>prompts</li>
      <li>conversation text</li>
      <li>source code</li>
      <li>absolute paths</li>
      <li>API keys</li>
      <li>raw local databases</li>
    </ul>
    <p v-if="msg">{{ msg }}</p>
    <p>
      <button type="button" :disabled="busy" @click="del('/api/v1/account/usage', '已删除云端用量，账号仍保留')">
        删除已同步数据
      </button>
    </p>
    <p>
      <button type="button" :disabled="busy" @click="del('/api/v1/account', '账号已删除')">
        删除账号
      </button>
    </p>
  </main>
</template>
