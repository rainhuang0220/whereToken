<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { isHosted } from '../mode'
import {
  PALETTE_LABEL,
  PUBLIC_PALETTES,
  applyPublication,
  bundlePalette,
  emptyPublication,
  heatHex,
  loadPublication,
  previewIntensities,
  publicationPending,
  statusCopy,
  type PublicPaletteId,
  type PublicationView,
} from './publicAppearance'

const hosted = isHosted()
const view = ref<PublicationView>(emptyPublication())
const selected = ref<PublicPaletteId>('newsprint')
const frame = ref<'desktop' | 'mobile'>('desktop')
const busy = ref(false)
const loadError = ref('')
const intensities = previewIntensities()
const wall = ref<HTMLElement | null>(null)

const stagePage = computed(() => (selected.value === 'newsprint' ? '#f7f6f1' : '#ffffff'))
const stageInk = computed(() => '#1f2328')
const stageMuted = computed(() => (selected.value === 'newsprint' ? '#4b4b4b' : '#57606a'))
const paper = computed(() =>
  selected.value === 'newsprint' && !hosted ? 'url("/api/public-profile/surface.jpg")' : 'none',
)
const shipped = computed(() => bundlePalette(view.value))
const pending = computed(() => publicationPending(selected.value, view.value))
const copy = computed(() => (loadError.value ? loadError.value : statusCopy(view.value)))

onMounted(() => {
  void refresh()
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
        <h2 id="public-face-h">公开 Profile</h2>
        <p>Cobalt / Magenta / Newsprint。本机釉色另算。</p>
      </div>
      <p class="public-face-state" role="status">
        <span>公开包 {{ shipped ? PALETTE_LABEL[shipped] : '未生成' }}</span>
        <span>编辑 {{ PALETTE_LABEL[selected] }}</span>
        <span v-if="pending">待发布</span>
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
      <button type="button" class="lever primary" :disabled="hosted || busy" @click="apply">
        {{ busy ? '正在写入' : '应用到公开 Profile' }}
      </button>
      <button type="button" class="lever" @click="preview">预览公开 Profile</button>
    </div>
    <p class="public-copy" role="status">{{ copy }}</p>
    <p class="public-command"><code>{{ view.export_command }}</code></p>
  </section>
</template>
