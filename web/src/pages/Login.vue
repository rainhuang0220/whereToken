<script setup lang="ts">
import KilnKid from '../components/KilnKid.vue'
import { safeReturnPath } from '../hosted/account'
import {
  githubLabel,
  githubRepoHref,
  loginCta,
  loginHeadline,
  loginLede,
  loginNoRepo,
  loginPrivacy,
  loginStatus,
  neverSyncedItems,
  oauthErrorBody,
  oauthErrorTitle,
  projectSiteHref,
  siteLabel,
  syncedItems,
  syncWhatTitle,
} from '../hosted/copy'

const params = new URLSearchParams(window.location.search)
const next = safeReturnPath(params.get('next'))
const oauthFailed = params.get('err') === 'oauth'
const requestId = params.get('rid') || ''

function github() {
  const q = next.startsWith('/') ? `?next=${encodeURIComponent(next)}` : ''
  window.location.assign(`/api/v1/auth/github${q}`)
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
        <p class="status-line">{{ loginStatus }}</p>
        <div class="rail-actions">
          <router-link class="lever" to="/themes">主题</router-link>
        </div>
      </div>
    </header>

    <p class="cold-copy">{{ loginHeadline }}</p>
    <p class="note">{{ loginLede }}</p>

    <section v-if="oauthFailed" class="cold-kiln" role="alert">
      <div>
        <p class="cold-kicker">{{ oauthErrorTitle }}</p>
        <p class="cold-copy">{{ oauthErrorBody }}</p>
        <p v-if="requestId" class="status-line">Reference: {{ requestId }}</p>
        <div class="damper">
          <button type="button" class="lever primary" @click="github">重新登录</button>
          <a class="ext" :href="projectSiteHref" target="_blank" rel="noopener noreferrer">{{ siteLabel }} ↗</a>
        </div>
      </div>
    </section>

    <template v-else>
      <div class="damper">
        <button type="button" class="lever primary" @click="github">{{ loginCta }}</button>
      </div>
      <p class="status-line">{{ loginPrivacy }} · {{ loginNoRepo }}</p>
      <p class="cold-kicker">{{ syncWhatTitle }}</p>
      <p class="note">✓ {{ syncedItems.join(' · ') }}</p>
      <p class="note">✗ {{ neverSyncedItems.join(' · ') }}</p>
    </template>

    <p class="status-line login-foot">
      <a class="ext" :href="projectSiteHref" target="_blank" rel="noopener noreferrer">{{ siteLabel }} ↗</a>
      <span class="status-dot" aria-hidden="true">·</span>
      <a class="ext" :href="githubRepoHref" target="_blank" rel="noopener noreferrer">{{ githubLabel }} ↗</a>
    </p>
  </div>
</template>
