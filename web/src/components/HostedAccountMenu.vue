<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { csrfHeaders } from '../csrf'
import {
  avatarInitial,
  displayHandle,
  firstMenuIndex,
  lastMenuIndex,
  menuOffset,
  moveMenuIndex,
  triggerMenuKey,
  type MenuItem,
} from '../hosted/account'
import { loginCta, projectSiteHref, siteLabel } from '../hosted/copy'

const router = useRouter()
const login = ref('')
const avatar = ref('')
const loaded = ref(false)
const broken = ref(false)
const open = ref(false)
const active = ref(0)
const fail = ref('')
const triggerEl = ref<HTMLButtonElement | null>(null)
const menuEl = ref<HTMLElement | null>(null)
const menuPos = ref({ top: 0, left: 0 })
const placed = ref(false)

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
  placed.value = false
  fail.value = ''
  if (restore) triggerEl.value?.focus()
}

function place() {
  const t = triggerEl.value?.getBoundingClientRect()
  const m = menuEl.value?.getBoundingClientRect()
  if (!t || !m) return
  menuPos.value = menuOffset(t, { width: m.width, height: m.height }, {
    width: window.innerWidth,
    height: window.innerHeight,
  })
}

async function openAt(index: number) {
  open.value = true
  placed.value = false
  fail.value = ''
  active.value = index
  await nextTick()
  place()
  placed.value = true
  focusActive()
}

function toggle() {
  if (open.value) {
    close()
    return
  }
  void openAt(firstMenuIndex(items))
}

function onTriggerKey(e: KeyboardEvent) {
  const act = triggerMenuKey(open.value, e.key)
  if (!act) return
  e.preventDefault()
  if (act === 'close') {
    close()
    return
  }
  if (act === 'toggle') {
    toggle()
    return
  }
  void openAt(act === 'open-last' ? lastMenuIndex(items) : firstMenuIndex(items))
}

function onMenuKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
    return
  }
  if (e.key === 'Tab') {
    close(false)
    return
  }
  if (e.key === 'Home') {
    e.preventDefault()
    active.value = firstMenuIndex(items)
    focusActive()
    return
  }
  if (e.key === 'End') {
    e.preventDefault()
    active.value = lastMenuIndex(items)
    focusActive()
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

onMounted(() => {
  document.addEventListener('mousedown', onDoc)
  window.addEventListener('resize', onWin)
})
onUnmounted(() => {
  document.removeEventListener('mousedown', onDoc)
  window.removeEventListener('resize', onWin)
})

function onWin() {
  if (open.value) place()
}

async function logout() {
  fail.value = ''
  const res = await fetch('/api/v1/auth/logout', {
    method: 'POST',
    credentials: 'same-origin',
    headers: csrfHeaders(),
  })
  if (!res.ok) {
    fail.value = '退出失败，请再试一次。不会断开 CLI 设备。'
    return
  }
  close(false)
  window.location.assign('/login')
}

function go(path: string) {
  close(false)
  void router.push(path)
}
</script>

<template>
  <div class="acct">
    <span v-if="!loaded" class="acct-trigger is-skel" aria-hidden="true">
      <span class="acct-avatar" />
      <span class="acct-name">&nbsp;</span>
    </span>
    <a v-else-if="!login" class="lever" href="/login">{{ loginCta }}</a>
    <template v-else>
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
        :class="{ 'is-placed': placed }"
        role="menu"
        :style="placed ? { top: menuPos.top + 'px', left: menuPos.left + 'px' } : undefined"
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
          :href="projectSiteHref"
          target="_blank"
          rel="noopener noreferrer"
          @click="close(false)"
        >
          {{ siteLabel }} ↗
        </a>
        <div class="acct-sep" role="separator" />
        <button type="button" class="acct-item" role="menuitem" @click="logout">退出登录</button>
        <p v-if="fail" class="acct-fail" role="alert">{{ fail }}</p>
      </div>
    </template>
  </div>
</template>
