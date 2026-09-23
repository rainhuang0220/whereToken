<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
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
const copy = computed(() => (loadError.value ? loadError.value : statusCopy(view.value)))
const phaseLabel = computed(() => job.value?.phase_label || preflight.value?.phase_label || '')
const running = computed(() => publishRunning(job.value?.phase))
const desktopLink = computed(() => handoffURL(selected.value))

onMounted(() => {
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
    loadError.value = '读不到本机公开 Profile 配置。'
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
    if (view.value.status === 'failed') loadError.value = view.value.error || '写入失败。'
  } catch {
    loadError.value = '写入失败。'
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
    publishError.value = '预检失败。确认本机 wheretoken 正在运行。'
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
    publishError.value = '发布请求被拒绝。'
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
    publishError.value = '重试被拒绝。请重新预检。'
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
    loadError.value = '还没有本地公开包。上面是外观预览。'
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
        <h2 id="public-face-h">公开 Profile 外观</h2>
        <p>选 Cobalt、Magenta 或 Newsprint。仅保存本机不会上线。发布到 GitHub 主页需要再确认一次。</p>
      </div>
      <p class="public-face-state" role="status">
        <span>公开包 {{ shipped ? PALETTE_LABEL[shipped] : '未生成' }}</span>
        <span>编辑 {{ PALETTE_LABEL[selected] }}</span>
        <span v-if="phaseLabel">{{ phaseLabel }}</span>
        <span v-else-if="pending">未保存</span>
      </p>
    </header>

    <div class="public-choices" role="radiogroup" aria-label="公开 Profile 外观">
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

    <div class="public-stage-tools" role="group" aria-label="预览尺寸">
      <button type="button" :aria-pressed="frame === 'desktop'" @click="frame = 'desktop'">桌面</button>
      <button type="button" :aria-pressed="frame === 'mobile'" @click="frame = 'mobile'">手机</button>
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
        <span>外观预览</span>
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
        {{ busy ? '正在写入' : '仅保存本机' }}
      </button>
      <button type="button" class="lever primary" :disabled="hosted || running" @click="openPreflight">发布到我的 GitHub 主页</button>
      <button type="button" class="lever" @click="preview">预览公开 Profile</button>
    </div>
    <section v-if="preflight && !hosted" class="public-preflight" aria-live="polite">
      <p>{{ phaseLabel }}</p>
      <p>选择 {{ preflight.palette }}。线上现在是 {{ preflight.live_palette || '还没读到' }}。</p>
      <p>{{ preflight.product_repo }} {{ preflight.product_branch }} · public-profile/</p>
      <p>{{ preflight.profile_repo }} {{ preflight.profile_branch }} · {{ preflight.readme_path }}</p>
      <p v-if="preflight.github_login">GitHub {{ preflight.github_login }}</p>
      <p v-if="preflight.validation">校验 {{ preflight.validation }} · {{ preflight.provenance }}</p>
      <p v-if="preflight.snapshot_id">snapshot {{ preflight.snapshot_id }}</p>
      <p v-if="preflight.cache_key">缓存 {{ preflight.cache_key }}</p>
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
      <p v-for="item in preflight.ahead || []" :key="item">将一并推送 {{ item }}</p>
      <p v-for="block in preflight.blockers || []" :key="block">{{ block }}</p>
      <p v-if="preflight.static_svg">GitHub 主页上的图是静态 SVG。实时纸面只在打开的公开页里。</p>
      <p>{{ preflight.release_note }}</p>
      <button v-if="preflight.ready && !running" type="button" class="lever primary" @click="confirmPublish">确认发布到这两个仓库</button>
      <button v-if="job?.retry_readme" type="button" class="lever" @click="retryReadme">重试更新 GitHub 主页</button>
      <p v-if="job?.product_url"><a :href="job.product_url">whereToken 提交</a></p>
      <p v-if="job?.pages_run_url"><a :href="job.pages_run_url">Pages</a></p>
      <p v-if="job?.profile_url"><a :href="job.profile_url">个人 README 提交</a></p>
      <p v-if="job?.live_page && job.phase === 'verified'"><a :href="job.live_page">公开页</a></p>
      <p v-if="job?.error">{{ job.error }}</p>
      <p v-if="publishError">{{ publishError }}</p>
    </section>
    <p v-if="hosted" class="public-copy">这个演示站不能发布。在已经运行 wheretoken serve 的电脑上打开下面的链接。手机上的 127.0.0.1 不是那台电脑。</p>
    <p v-if="hosted" class="public-command"><code>{{ desktopLink }}</code></p>
    <p class="public-copy" role="status">{{ copy }}</p>
    <p class="public-command"><code>{{ view.export_command }}</code></p>
  </section>
</template>
