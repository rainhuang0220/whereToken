<script setup lang="ts">
import HostedShell from '../components/HostedShell.vue'
import {
  loginCta,
  loginHeadline,
  loginLede,
  loginNoRepo,
  loginPrivacy,
  oauthErrorBody,
  oauthErrorTitle,
} from '../hosted/copy'

const params = new URLSearchParams(window.location.search)
const next = params.get('next') || '/app'
const oauthFailed = params.get('err') === 'oauth'
const requestId = params.get('rid') || ''

function github() {
  const q = next.startsWith('/') ? `?next=${encodeURIComponent(next)}` : ''
  window.location.assign(`/api/v1/auth/github${q}`)
}
</script>

<template>
  <HostedShell bare>
    <main class="hosted-gate">
      <p class="whisper">Hosted</p>
      <h1>{{ loginHeadline }}</h1>
      <p class="hosted-lede">{{ loginLede }}</p>

      <section v-if="oauthFailed" class="hosted-error" role="alert">
        <h2>{{ oauthErrorTitle }}</h2>
        <p>{{ oauthErrorBody }}</p>
        <p v-if="requestId" class="hosted-ref">Reference: <code>{{ requestId }}</code></p>
        <div class="hosted-cta-row">
          <button type="button" class="lever primary" @click="github">重新登录</button>
          <a class="lever" href="https://rainhuang0220.github.io/whereToken/">返回首页</a>
        </div>
      </section>

      <template v-else>
        <section class="hosted-preview" aria-label="Preview">
          <p class="cold-kicker">Preview</p>
          <p class="hosted-preview-note">登录后看到的是你自己的账本，不是这段示意骨架。</p>
          <div class="hosted-skel" aria-hidden="true">
            <span /><span /><span /><span />
          </div>
        </section>

        <div class="hosted-cta">
          <button type="button" class="lever primary hosted-github" @click="github">
            {{ loginCta }}
          </button>
          <p class="hosted-reassure">{{ loginPrivacy }} · {{ loginNoRepo }}</p>
        </div>

        <details class="kiln-mouth hosted-sync">
          <summary>What gets synced?</summary>
          <ul class="hosted-yes">
            <li>token counts</li>
            <li>models &amp; vendors</li>
            <li>request statistics</li>
          </ul>
          <ul class="hosted-no">
            <li>prompts</li>
            <li>source code</li>
            <li>file paths</li>
            <li>API keys</li>
          </ul>
        </details>
      </template>

      <footer class="hosted-foot">
        <a href="https://rainhuang0220.github.io/whereToken/">Project Site</a>
        <a href="https://github.com/rainhuang0220/whereToken">GitHub</a>
        <router-link to="/settings/privacy">Privacy</router-link>
      </footer>
    </main>
  </HostedShell>
</template>
