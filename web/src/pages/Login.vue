<script setup lang="ts">
const next = new URLSearchParams(window.location.search).get('next') || '/app'
function github() {
  const q = next.startsWith('/') ? `?next=${encodeURIComponent(next)}` : ''
  window.location.assign(`/api/v1/auth/github${q}`)
}
</script>

<template>
  <main class="page">
    <h1>whereToken</h1>
    <p>登录后绑定本机 CLI，查看你自己的真实用量。这里不会塞试用样本。</p>
    <button type="button" class="btn" @click="github">使用 GitHub 登录</button>
  </main>
</template>
