<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { csrfHeaders } from '../csrf'
import KilnKid from './KilnKid.vue'

const props = defineProps<{ bare?: boolean }>()
const login = ref('')
const avatar = ref('')

onMounted(async () => {
  if (props.bare) return
  const res = await fetch('/api/v1/session', { credentials: 'same-origin' })
  if (!res.ok) return
  const body = (await res.json()) as { login?: string; avatar_url?: string }
  login.value = body.login || ''
  avatar.value = body.avatar_url || ''
})

async function logout() {
  await fetch('/api/v1/auth/logout', {
    method: 'POST',
    credentials: 'same-origin',
    headers: csrfHeaders(),
  })
  window.location.assign('/login')
}
</script>

<template>
  <div class="hosted-shell">
    <header class="hosted-top">
      <router-link class="hosted-brand" to="/app">
        <KilnKid pose="grin" size="sm" />
        <span>whereToken</span>
      </router-link>
      <nav v-if="!bare && login" class="hosted-nav" aria-label="Hosted">
        <router-link to="/app">Dashboard</router-link>
        <router-link to="/settings/devices">Devices</router-link>
        <router-link to="/settings/privacy">Privacy</router-link>
        <router-link to="/themes">主题</router-link>
      </nav>
      <div v-if="!bare && login" class="hosted-user">
        <img v-if="avatar" class="hosted-avatar" :src="avatar" :alt="login" width="24" height="24" />
        <span class="hosted-login">{{ login }}</span>
        <button type="button" class="lever" @click="logout">退出</button>
      </div>
      <router-link v-else-if="bare" class="lever" to="/themes">主题</router-link>
    </header>
    <slot />
  </div>
</template>
