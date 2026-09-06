<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { csrfHeaders } from '../csrf'
import { avatarInitial, displayHandle, moveMenuIndex, type MenuItem } from '../hosted/account'

const router = useRouter()
const login = ref('')
const avatar = ref('')
const loaded = ref(false)
const broken = ref(false)
const open = ref(false)
const active = ref(0)
const triggerEl = ref<HTMLButtonElement | null>(null)
const menuEl = ref<HTMLElement | null>(null)

const handle = computed(() => displayHandle(login.value))
const initial = computed(() => avatarInitial(login.value))
const items: MenuItem[] = [
  { id: 'dash', kind: 'item' },
  { id: 'devices', kind: 'item' },
  { id: 'settings', kind: 'item' },
  { id: 'sep1', kind: 'sep' },
  { id: 'site', kind: 'item' },
  { id: 'sep2', kind: 'sep' },
  { id: 'logout', kind: 'item' },
]

onMounted(async () => {
  const res = await fetch('/api/v1/session', { credentials: 'same-origin' })
  loaded.value = true
  if (!res.ok) return
  const body = (await res.json()) as { login?: string; avatar_url?: string }
  login.value = body.login || ''
  avatar.value = body.avatar_url || ''
})

function close(restore = true) {
  open.value = false
  if (restore) triggerEl.value?.focus()
}

function toggle() {
  open.value = !open.value
  if (open.value) {
    active.value = 0
    void nextTick(() => menuEl.value?.querySelector<HTMLElement>('[role="menuitem"]')?.focus())
  }
}

function onTriggerKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ') {
    e.preventDefault()
    if (!open.value) toggle()
  }
}

function onMenuKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = moveMenuIndex(items, active.value, 1)
    focusActive()
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = moveMenuIndex(items, active.value, -1)
    focusActive()
  }
}

function focusActive() {
  const nodes = menuEl.value?.querySelectorAll<HTMLElement>('[role="menuitem"]')
  const ids = items.map((it, i) => (it.kind === 'item' ? i : -1)).filter((i) => i >= 0)
  const slot = ids.indexOf(active.value)
  nodes?.[slot]?.focus()
}

function onDoc(e: MouseEvent) {
  const t = e.target as Node | null
  if (!t) return
  if (triggerEl.value?.contains(t) || menuEl.value?.contains(t)) return
  close(false)
}

onMounted(() => document.addEventListener('mousedown', onDoc))
onUnmounted(() => document.removeEventListener('mousedown', onDoc))

async function logout() {
  close(false)
  await fetch('/api/v1/auth/logout', {
    method: 'POST',
    credentials: 'same-origin',
    headers: csrfHeaders(),
  })
  window.location.assign('/login')
}

function go(path: string) {
  close(false)
  void router.push(path)
}
</script>

<template>
  <div v-if="loaded && login" class="acct">
    <button
      ref="triggerEl"
      type="button"
      class="acct-trigger"
      :aria-expanded="open"
      aria-haspopup="menu"
      aria-controls="acct-menu"
      :aria-label="'打开 ' + handle + ' 的账户菜单'"
      @click="toggle"
      @keydown="onTriggerKey"
    >
      <img
        v-if="avatar && !broken"
        class="acct-avatar"
        :src="avatar"
        alt=""
        width="24"
        height="24"
        @error="broken = true"
      />
      <span v-else class="acct-avatar acct-fallback" aria-hidden="true">{{ initial }}</span>
      <span class="acct-name">{{ handle }}</span>
      <span class="acct-caret" aria-hidden="true">▾</span>
    </button>
    <div
      v-if="open"
      id="acct-menu"
      ref="menuEl"
      class="acct-menu"
      role="menu"
      @keydown="onMenuKey"
    >
      <div class="acct-id">
        <img
          v-if="avatar && !broken"
          class="acct-avatar"
          :src="avatar"
          alt=""
          width="24"
          height="24"
        />
        <span v-else class="acct-avatar acct-fallback" aria-hidden="true">{{ initial }}</span>
        <div>
          <p class="acct-id-name">{{ handle }}</p>
          <p class="acct-id-note">GitHub 账号</p>
        </div>
      </div>
      <button type="button" class="acct-item" role="menuitem" @click="go('/app')">仪表盘</button>
      <button type="button" class="acct-item" role="menuitem" @click="go('/settings/devices')">设备</button>
      <button type="button" class="acct-item" role="menuitem" @click="go('/settings')">设置</button>
      <div class="acct-sep" role="separator" />
      <a
        class="acct-item"
        role="menuitem"
        href="https://rainhuang0220.github.io/whereToken/"
        target="_blank"
        rel="noopener noreferrer"
        @click="close(false)"
      >
        项目主页 ↗
      </a>
      <div class="acct-sep" role="separator" />
      <button type="button" class="acct-item" role="menuitem" @click="logout">退出登录</button>
    </div>
  </div>
</template>
