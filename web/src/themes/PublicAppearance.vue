<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { persistLang, resolveLang, t, type Lang } from '../i18n'
import { isHosted } from '../mode'
import {
  PALETTE_LABEL,
  PUBLIC_PALETTES,
  applyPublication,
  bundlePalette,
  emptyPublication,
  handoffFromQuery,
  handoffURL,
  heatHex,
  loadPreflight,
  loadPublication,
  loadPublishJob,
  postPublish,
  previewIntensities,
  publicationPending,
  publishRunning,
  statusCopy,
  type PublicPaletteId,
  type PublicationView,
  type PublishJob,
  type PublishPreflight,
} from './publicAppearance'

const hosted = isHosted()
const route = useRoute()
const router = useRouter()
const lang = ref<Lang>('en')
const view = ref<PublicationView>(emptyPublication())
const selected = ref<PublicPaletteId>('newsprint')
const frame = ref<'desktop' | 'mobile'>('desktop')
const busy = ref(false)
const loadError = ref('')
const intensities = previewIntensities()
const wall = ref<HTMLElement | null>(null)
const preflight = ref<PublishPreflight | null>(null)
const job = ref<PublishJob | null>(null)
const publishError = ref('')
let poll = 0

const stagePage = computed(() => (selected.value === 'newsprint' ? '#f7f6f1' : '#ffffff'))
const stageInk = computed(() => '#1f2328')
const stageMuted = computed(() => (selected.value === 'newsprint' ? '#4b4b4b' : '#57606a'))
const paper = computed(() =>
  selected.value === 'newsprint' && !hosted ? 'url("/api/public-profile/surface.jpg")' : 'none',
)
const shipped = computed(() => bundlePalette(view.value))
const pending = computed(() => publicationPending(selected.value, view.value))
const copy = computed(() => (loadError.value ? loadError.value : statusCopy(view.value, lang.value)))
const phaseLabel = computed(() => {
  const phase = job.value?.phase || preflight.value?.phase || ''
  const fallback = job.value?.phase_label || preflight.value?.phase_label || ''
  if (!phase) return ''
  const text = t(lang.value, 'phase.' + phase)
  return text === 'phase.' + phase ? fallback : text
})
const running = computed(() => publishRunning(job.value?.phase))
const desktopLink = computed(() => handoffURL(selected.value))

function tr(key: string, vars?: Record<string, string>) {
  return t(lang.value, key, vars)
}

function chooseLang(next: Lang) {
  lang.value = next
  persistLang(next)
  void router.replace({ query: { ...route.query, lang: next }, hash: route.hash })
}

onMounted(() => {
  lang.value = resolveLang(window.location.search)
  void boot()
})

onBeforeUnmount(() => {
  window.clearInterval(poll)
})

watch(selected, (value) => {
  if (preflight.value && preflight.value.palette !== value) {
    preflight.value = null
    job.value = null
    publishError.value = ''
  }
})

watch(frame, async () => {
  await nextTick()
  scrollWall()
})

function scrollWall() {
  const node = wall.value
  if (!node) return
  node.scrollLeft = node.scrollWidth
}

async function boot() {
  await refresh()
  const handoff = handoffFromQuery(route.query as Record<string, unknown>)
  if (handoff.palette) selected.value = handoff.palette
  if (handoff.publish && !hosted) await openPreflight()
}

async function refresh() {
  if (hosted) {
    view.value = { ...emptyPublication(), status: 'hosted' }
    selected.value = 'newsprint'
    return
  }
  try {
    const next = await loadPublication(false)
    view.value = next
    if (!busy.value) selected.value = next.public_palette
    loadError.value = ''
    await nextTick()
    scrollWall()
  } catch {
    loadError.value = tr('appearance.readFailed')
    view.value = { ...view.value, status: 'failed', error: loadError.value }
  }
}

async function apply() {
  if (hosted || busy.value) return
  busy.value = true
  loadError.value = ''
  view.value = { ...view.value, status: 'loading' }
  try {
    view.value = await applyPublication(selected.value)
    if (view.value.status === 'failed') loadError.value = view.value.error || tr('appearance.writeFailed')
  } catch {
    loadError.value = tr('appearance.writeFailed')
    view.value = { ...view.value, status: 'failed', error: loadError.value }
  } finally {
    busy.value = false
  }
}

async function openPreflight() {
  if (hosted || busy.value) return
  publishError.value = ''
  job.value = null
  try {
    preflight.value = await loadPreflight(selected.value)
  } catch {
    publishError.value = tr('appearance.preflightFailed')
  }
}

async function confirmPublish() {
  const current = preflight.value
  if (!current?.ready || hosted || running.value) return
  publishError.value = ''
  try {
    job.value = await postPublish(current, 'approve')
    startPoll()
  } catch {
    publishError.value = tr('appearance.publishRefused')
  }
}

async function retryReadme() {
  const current = preflight.value
  if (!current || hosted) return
  publishError.value = ''
  try {
    job.value = await postPublish(current, 'retry_readme')
    startPoll()
  } catch {
    publishError.value = tr('appearance.retryRefused')
  }
}

function startPoll() {
  window.clearInterval(poll)
  poll = window.setInterval(() => {
    void loadPublishJob()
      .then((next) => {
        if (!next) return
        job.value = next
        if (!publishRunning(next.phase)) {
          window.clearInterval(poll)
          void refresh()
        }
      })
      .catch(() => {
        window.clearInterval(poll)
      })
  }, 2000)
}

function preview() {
  if (!view.value.preview_path) {
    loadError.value = tr('appearance.noBundle')
    return
  }
  const url = new URL(view.value.preview_path, window.location.origin)
  if (selected.value !== shipped.value) url.searchParams.set('palette', selected.value)
  window.open(url.toString(), '_blank', 'noopener')
}

function onPaletteKey(event: KeyboardEvent, index: number) {
  let next = index
  if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = (index + 1) % PUBLIC_PALETTES.length
  else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = (index - 1 + PUBLIC_PALETTES.length) % PUBLIC_PALETTES.length
  else if (event.key === 'Home') next = 0
  else if (event.key === 'End') next = PUBLIC_PALETTES.length - 1
  else return
  event.preventDefault()
  selected.value = PUBLIC_PALETTES[next]
  const buttons = (event.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLButtonElement>('[role="radio"]')
  buttons?.[next]?.focus()
}

function emptyOf(id: PublicPaletteId): string {
  return id === 'newsprint' ? '#e8e8e6' : '#eaeef2'
}

function paint(id: PublicPaletteId, intensity: number): Record<string, string> {
  if (intensity <= 0) return { background: emptyOf(id) }
  return { background: heatHex(id, intensity) }
}
</script>

<template>
  <section class="public-face" aria-labelledby="public-face-h">
    <header class="public-face-head">
      <div>
        <h2 id="public-face-h">{{ tr('appearance.h') }}</h2>
        <p>{{ tr('appearance.lead') }}</p>
      </div>
      <div class="public-stage-tools" role="group" :aria-label="tr('lang.group')">
        <button type="button" :aria-pressed="lang === 'en'" @click="chooseLang('en')">EN</button>
        <button type="button" :aria-pressed="lang === 'zh-CN'" @click="chooseLang('zh-CN')">中文</button>
      </div>
      <p class="public-face-state" role="status">
        <span>{{ tr('appearance.bundle', { name: shipped ? PALETTE_LABEL[shipped] : tr('appearance.bundleMissing') }) }}</span>
        <span>{{ tr('appearance.editing', { name: PALETTE_LABEL[selected] }) }}</span>
        <span v-if="phaseLabel">{{ phaseLabel }}</span>
        <span v-else-if="pending">{{ tr('appearance.unsaved') }}</span>
      </p>
    </header>

    <div class="public-choices" role="radiogroup" :aria-label="tr('appearance.group')">
      <button
        v-for="(id, index) in PUBLIC_PALETTES"
        :key="id"
        type="button"
        class="public-choice"
        role="radio"
        :aria-checked="selected === id"
        :tabindex="selected === id ? 0 : -1"
        @click="selected = id"
        @keydown="onPaletteKey($event, index)"
      >
        <span class="public-choice-name">{{ PALETTE_LABEL[id] }}</span>
        <span class="public-mini" aria-hidden="true">
          <i v-for="(intensity, cell) in intensities" :key="cell" :style="paint(id, intensity)" />
        </span>
      </button>
    </div>

    <div class="public-stage-tools" role="group" :aria-label="tr('appearance.frame')">
      <button type="button" :aria-pressed="frame === 'desktop'" @click="frame = 'desktop'">{{ tr('appearance.desktop') }}</button>
      <button type="button" :aria-pressed="frame === 'mobile'" @click="frame = 'mobile'">{{ tr('appearance.mobile') }}</button>
    </div>

    <div
      class="public-stage"
      :class="frame"
      :style="{
        '--stage-page': stagePage,
        '--stage-ink': stageInk,
        '--stage-muted': stageMuted,
        '--stage-paper': paper,
      }"
    >
      <div class="public-stage-top">
        <strong>whereToken</strong>
        <span>{{ tr('appearance.previewLabel') }}</span>
      </div>
      <p class="public-stage-total">—</p>
      <div ref="wall" class="public-wall-scroll">
        <div class="public-wall" aria-hidden="true">
          <i v-for="(intensity, cell) in intensities" :key="cell" :style="paint(selected, intensity)" />
        </div>
      </div>
    </div>

    <div class="public-actions">
      <button type="button" class="lever" :disabled="hosted || busy" @click="apply">
        {{ busy ? tr('appearance.saving') : tr('appearance.save') }}
      </button>
      <button type="button" class="lever primary" :disabled="hosted || running" @click="openPreflight">{{ tr('appearance.publish') }}</button>
      <button type="button" class="lever" @click="preview">{{ tr('appearance.preview') }}</button>
    </div>
    <section v-if="preflight && !hosted" class="public-preflight" aria-live="polite">
      <p>{{ phaseLabel }}</p>
      <p>{{ tr('appearance.selected', { palette: preflight.palette, live: preflight.live_palette || tr('appearance.liveUnknown') }) }}</p>
      <p>{{ preflight.product_repo }} {{ preflight.product_branch }} · public-profile/</p>
      <p>{{ preflight.profile_repo }} {{ preflight.profile_branch }} · {{ preflight.readme_path }}</p>
      <p v-if="preflight.github_login">GitHub {{ preflight.github_login }}</p>
      <p v-if="preflight.validation">{{ tr('appearance.check', { validation: preflight.validation, provenance: preflight.provenance || '' }) }}</p>
      <p v-if="preflight.snapshot_id">snapshot {{ preflight.snapshot_id }}</p>
      <p v-if="preflight.cache_key">{{ tr('appearance.cache', { key: preflight.cache_key }) }}</p>
      <ul v-if="preflight.files?.length">
        <li v-for="file in preflight.files" :key="file.path">{{ file.action }} {{ file.path }}</li>
      </ul>
      <ul v-if="preflight.readme_edits?.length">
        <li v-for="edit in preflight.readme_edits" :key="edit.slot">
          {{ edit.slot }}
          <code>{{ edit.before }}</code>
          <code>{{ edit.after }}</code>
        </li>
      </ul>
      <p v-for="item in preflight.ahead || []" :key="item">{{ tr('appearance.alsoPush', { item }) }}</p>
      <p v-for="block in preflight.blockers || []" :key="block">{{ block }}</p>
      <p v-if="preflight.static_svg">{{ tr('appearance.staticSvg') }}</p>
      <p>{{ preflight.release_note }}</p>
      <button v-if="preflight.ready && !running" type="button" class="lever primary" @click="confirmPublish">{{ tr('appearance.confirm') }}</button>
      <button v-if="job?.retry_readme" type="button" class="lever" @click="retryReadme">{{ tr('appearance.retry') }}</button>
      <p v-if="job?.product_url"><a :href="job.product_url">{{ tr('appearance.productCommit') }}</a></p>
      <p v-if="job?.pages_run_url"><a :href="job.pages_run_url">{{ tr('appearance.pages') }}</a></p>
      <p v-if="job?.profile_url"><a :href="job.profile_url">{{ tr('appearance.readmeCommit') }}</a></p>
      <p v-if="job?.live_page && job.phase === 'verified'"><a :href="job.live_page">{{ tr('appearance.livePage') }}</a></p>
      <p v-if="job?.error">{{ job.error }}</p>
      <p v-if="publishError">{{ publishError }}</p>
    </section>
    <p v-if="hosted" class="public-copy">{{ tr('appearance.hosted') }}</p>
    <p v-if="hosted" class="public-command"><code>{{ desktopLink }}</code></p>
    <p class="public-copy" role="status">{{ copy }}</p>
    <p class="public-command"><code>{{ view.export_command }}</code></p>
  </section>
</template>
