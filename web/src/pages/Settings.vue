<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { fetchSummary } from '../api'
import CopyCommand from '../components/CopyCommand.vue'
import HostedAccountMenu from '../components/HostedAccountMenu.vue'
import KilnKid from '../components/KilnKid.vue'
import { csrfHeaders } from '../csrf'
import { avatarInitial, displayHandle, isUnauthorized, safeReturnPath } from '../hosted/account'
import { neverSyncedItems, syncedItems } from '../hosted/copy'
import { STORAGE_KEY, applyTheme, resolveThemeId, themes, type ThemeId } from '../themes'
import type { SummaryPayload } from '../types'

const route = useRoute()
const router = useRouter()
const tab = computed(() => {
  const s = String(route.params.section || 'account')
  return ['account', 'appearance', 'devices', 'privacy'].includes(s) ? s : 'account'
})

const login = ref('')
const avatar = ref('')
const broken = ref(false)
const rows = ref<NonNullable<NonNullable<SummaryPayload['hosted']>['devices']>>([])
const lastSync = ref('')
const error = ref('')
const msg = ref('')
const busy = ref(false)
const pending = ref<'usage' | 'account' | 'revoke' | ''>('')
const pendingId = ref('')
const themeId = ref<ThemeId>(resolveThemeId(null))

onMounted(async () => {
  try {
    themeId.value = resolveThemeId(localStorage.getItem(STORAGE_KEY))
  } catch {
    themeId.value = resolveThemeId(null)
  }
  const me = await fetch('/api/v1/session', { credentials: 'same-origin' })
  if (me.status === 401) {
    const next = encodeURIComponent(safeReturnPath(route.fullPath))
    window.location.assign(`/login?next=${next}`)
    return
  }
  if (me.ok) {
    const body = (await me.json()) as { login?: string; avatar_url?: string }
    login.value = body.login || ''
    avatar.value = body.avatar_url || ''
  }
  try {
    const p = await fetchSummary('all')
    rows.value = p.hosted?.devices ?? []
    lastSync.value = p.hosted?.last_sync_at || ''
  } catch (e) {
    if (isUnauthorized(e)) return
    error.value = e instanceof Error ? e.message : '无法加载数据'
  }
})

function go(section: string) {
  void router.push(section === 'account' ? '/settings' : `/settings/${section}`)
}

function setTheme(id: ThemeId) {
  themeId.value = id
  applyTheme(id)
}

function when(iso?: string) {
  if (!iso || iso.startsWith('0001')) return '—'
  return iso.replace('T', ' ').replace('Z', ' UTC')
}

async function logout() {
  const res = await fetch('/api/v1/auth/logout', {
    method: 'POST',
    credentials: 'same-origin',
    headers: csrfHeaders(),
  })
  if (!res.ok) {
    msg.value = '退出失败，请再试一次。不会断开 CLI 设备。'
    return
  }
  window.location.assign('/login')
}

async function revoke(id: string) {
  if (pending.value !== 'revoke' || pendingId.value !== id) {
    pending.value = 'revoke'
    pendingId.value = id
    msg.value = '再点一次以断开该设备。Web 登录不受影响。'
    return
  }
  busy.value = true
  const res = await fetch(`/api/v1/devices/${encodeURIComponent(id)}/revoke`, {
    method: 'POST',
    credentials: 'same-origin',
    headers: csrfHeaders(),
  })
  busy.value = false
  pending.value = ''
  pendingId.value = ''
  if (!res.ok) {
    msg.value = '断开设备失败'
    return
  }
  rows.value = rows.value.filter((d) => d.id !== id)
  msg.value = '已断开该设备的 CLI 令牌'
}

async function del(kind: 'usage' | 'account') {
  if (pending.value !== kind) {
    pending.value = kind
    msg.value = kind === 'usage' ? '再点一次以删除云端用量。设备仍保留。' : '再点一次将删除账号和全部云端数据。已连接的 CLI 会失效。'
    return
  }
  busy.value = true
  const path = kind === 'usage' ? '/api/v1/account/usage' : '/api/v1/account'
  const res = await fetch(path, {
    method: 'DELETE',
    credentials: 'same-origin',
    headers: csrfHeaders(),
  })
  busy.value = false
  pending.value = ''
  if (!res.ok) {
    msg.value = '操作失败'
    return
  }
  if (kind === 'account') {
    window.location.assign('/login')
    return
  }
  msg.value = '已删除云端用量，账号仍保留'
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
        <p class="status-line">设置</p>
        <div class="rail-actions">
          <router-link class="lever" to="/themes">主题</router-link>
          <HostedAccountMenu />
        </div>
      </div>
    </header>

    <nav class="damper" aria-label="设置">
      <button type="button" :class="{ on: tab === 'account' }" @click="go('account')">账户</button>
      <button type="button" :class="{ on: tab === 'appearance' }" @click="go('appearance')">外观</button>
      <button type="button" :class="{ on: tab === 'devices' }" @click="go('devices')">设备</button>
      <button type="button" :class="{ on: tab === 'privacy' }" @click="go('privacy')">同步与隐私</button>
    </nav>

    <p v-if="error" class="err" role="alert">{{ error }}</p>
    <p v-if="msg" class="note" role="status">{{ msg }}</p>

    <section v-if="tab === 'account'" class="why">
      <p class="cold-kicker">账户</p>
      <div class="acct-id is-static">
        <img
          v-if="avatar && !broken"
          class="acct-avatar"
          :src="avatar"
          alt=""
          width="24"
          height="24"
          @error="broken = true"
        />
        <span v-else class="acct-avatar acct-fallback" aria-hidden="true">{{ avatarInitial(login) }}</span>
        <div>
          <p class="acct-id-name">{{ displayHandle(login) }}</p>
          <p class="acct-id-note">通过 GitHub 识别，不能在 whereToken 里改名或换头像。</p>
        </div>
      </div>
      <div class="period">
        <button type="button" class="lever" @click="logout">退出登录</button>
      </div>
      <p class="note">退出只结束这个浏览器会话，不会断开已配对的 CLI 设备。</p>
    </section>

    <section v-else-if="tab === 'appearance'" class="why">
      <p class="cold-kicker">外观</p>
      <p class="note">使用现有釉色。完整预览走顶部「主题」。没有额外外观开关。</p>
      <div class="period">
        <button
          v-for="pack in themes"
          :key="pack.id"
          type="button"
          class="lever"
          :class="{ primary: themeId === pack.id }"
          @click="setTheme(pack.id)"
        >
          {{ pack.name }}
        </button>
      </div>
    </section>

    <section v-else-if="tab === 'devices'" class="why">
      <p class="cold-kicker">设备</p>
      <p class="note">断开设备会使该机器上的 CLI 令牌失效，不等于退出 Web。</p>
      <section v-if="!rows.length" class="cold-kiln">
        <div>
          <p class="cold-copy">尚未连接设备。</p>
          <CopyCommand command="wheretoken login" />
        </div>
      </section>
      <table v-else>
        <thead>
          <tr>
            <th class="name">设备</th>
            <th class="name">系统</th>
            <th class="name">版本</th>
            <th class="name">最近在线</th>
            <th class="name">最近同步</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in rows" :key="d.id">
            <td class="name">{{ d.label || 'unnamed' }}</td>
            <td class="name">{{ [d.os, d.arch].filter(Boolean).join(' · ') || '—' }}</td>
            <td class="name">{{ d.client_version || '—' }}</td>
            <td class="name">{{ when(d.last_seen) }}</td>
            <td class="name">{{ when(d.last_sync) }}</td>
            <td>
              <button type="button" class="lever" :disabled="busy" @click="revoke(d.id)">
                {{ pending === 'revoke' && pendingId === d.id ? '确认断开' : '断开设备' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <section v-else class="why">
      <p class="cold-kicker">同步与隐私</p>
      <p class="status-line">最近同步 {{ when(lastSync) }}</p>
      <p class="note">首次连接会自动同步。之后可运行 <code>wheretoken sync</code> 更新。浏览器不能替你扫描本机。</p>
      <CopyCommand command="wheretoken sync" />
      <p class="cold-kicker">会同步</p>
      <ul>
        <li v-for="item in syncedItems" :key="item">{{ item }}</li>
      </ul>
      <p class="cold-kicker">永不上传</p>
      <ul>
        <li v-for="item in neverSyncedItems" :key="item">{{ item }}</li>
      </ul>
      <p class="cold-kicker">危险操作</p>
      <div class="period">
        <button type="button" class="lever danger" :disabled="busy" @click="del('usage')">
          {{ pending === 'usage' ? '确认删除用量' : '删除已同步数据' }}
        </button>
        <button type="button" class="lever danger" :disabled="busy" @click="del('account')">
          {{ pending === 'account' ? '确认删除账号' : '删除账号' }}
        </button>
      </div>
    </section>
  </div>
</template>
