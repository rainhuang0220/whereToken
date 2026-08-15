<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  CHROME_TOKENS,
  REQUIRED_TOKENS,
  STORAGE_KEY,
  applyTheme,
  isThemeId,
  resolveThemeId,
  themes,
  type ThemeId,
  type ThemePack,
} from '../themes'
import {
  afterPaint,
  captureFlip,
  expandGallery,
  prefersReducedMotion,
  restoreGallery,
  settleGrid,
  type MotionHandle,
} from '../themes/galleryMotion'
import MockKeyboard from '../themes/MockKeyboard.vue'

const route = useRoute()
const router = useRouter()
const hall = ref<HTMLElement | null>(null)
const committed = ref<ThemeId>('kiln')
const openId = ref<ThemeId | null>(null)
const revealMock = ref(false)
let applied = false
let boot = true
let motion: MotionHandle | null = null
let motionGen = 0
let closing = false

onMounted(() => {
  let stored: string | null = null
  try {
    stored = localStorage.getItem(STORAGE_KEY)
  } catch {
    stored = null
  }
  committed.value = resolveThemeId(stored)
  boot = false
})

onBeforeUnmount(() => {
  motion?.revert()
  if (!applied) applyTheme(committed.value, { persist: false })
})

watch(
  () => route.params.id,
  async (raw) => {
    if (typeof raw === 'string' && raw && !isThemeId(raw)) {
      void router.replace('/themes')
      return
    }
    const id = typeof raw === 'string' && isThemeId(raw) ? raw : null
    if (id === openId.value) {
      if (id && closing) {
        closing = false
        motionGen += 1
        motion?.play()
      }
      return
    }
    if (id) await openCard(id, { instant: boot })
    else await closeCard({ instant: boot })
  },
  { immediate: true },
)

function slabs(): HTMLElement[] {
  return [...(hall.value?.querySelectorAll<HTMLElement>('.glaze-slab') ?? [])]
}

async function openCard(id: ThemeId, opts: { instant?: boolean } = {}) {
  const gen = ++motionGen
  closing = false
  revealMock.value = false
  motion?.revert()
  const reduced = Boolean(opts.instant) || prefersReducedMotion()
  const state = reduced ? null : captureFlip(slabs(), 'rest')
  openId.value = id
  await nextTick()
  await afterPaint()
  if (gen !== motionGen) return
  const hero = hall.value?.querySelector<HTMLElement>(`.glaze-slab[data-id="${id}"]`)
  if (!hero) return
  const others = slabs().filter((el) => el !== hero)
  motion = expandGallery({
    hero,
    others,
    reduced,
    state,
    onSettled() {
      if (gen !== motionGen) return
      revealMock.value = true
      applyTheme(id, { persist: false })
      void nextTick().then(() => {
        hall.value?.querySelector<HTMLButtonElement>('.glaze-back')?.focus()
      })
    },
  })
}

async function closeCard(opts: { instant?: boolean } = {}) {
  const gen = ++motionGen
  const id = openId.value
  if (!id) return
  const hero = hall.value?.querySelector<HTMLElement>(`.glaze-slab[data-id="${id}"]`)
  const others = slabs().filter((el) => el !== hero)
  const reduced = Boolean(opts.instant) || prefersReducedMotion()
  revealMock.value = false
  closing = true

  if (!reduced && motion?.canReverse()) {
    await nextTick()
    await afterPaint()
    if (gen !== motionGen) return
    applyTheme(committed.value, { persist: false })
    await afterPaint()
    if (gen !== motionGen) return
    await motion.reverse()
    if (gen !== motionGen) return
    settleGrid(hero, others)
    openId.value = null
    motion.revert()
    motion = null
    closing = false
    return
  }

  motion?.revert()
  applyTheme(committed.value, { persist: false })
  const state = reduced || !hero ? null : captureFlip(hero)
  openId.value = null
  await nextTick()
  await afterPaint()
  if (gen !== motionGen) return
  if (hero) motion = restoreGallery({ hero, others, reduced, state })
  closing = false
}

function onSlabClick(id: ThemeId) {
  if (openId.value) return
  void router.push(`/themes/${id}`)
}

function backToGrid() {
  const st = router.options.history.state as { back?: unknown }
  if (st.back != null) router.back()
  else void router.replace('/themes')
}

function commit() {
  if (!openId.value) return
  applied = true
  applyTheme(openId.value)
  void router.push('/')
}

function slabVars(t: ThemePack): Record<string, string> {
  const vars: Record<string, string> = {
    background: t.tokens.void,
    color: t.tokens.bone,
    borderColor: t.tokens.copper,
  }
  for (const key of REQUIRED_TOKENS) {
    if (key === 'scheme') continue
    vars[`--${key}`] = t.tokens[key]
  }
  for (const key of CHROME_TOKENS) {
    vars[`--${key}`] = t.chrome[key]
  }
  return vars
}

function onSlabKey(e: KeyboardEvent, id: ThemeId) {
  if (openId.value) return
  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault()
    onSlabClick(id)
  }
}
</script>

<template>
  <div ref="hall" class="glaze-hall" :class="{ 'is-open': openId }">
    <header class="rail glaze-head" :inert="Boolean(openId)">
      <h1>釉</h1>
      <div class="rail-meta">
        <p class="when">点一块看整页。应用才带走。</p>
        <div class="rail-actions">
          <router-link class="lever" to="/">返回</router-link>
        </div>
      </div>
    </header>

    <div class="glaze-shelf">
      <div v-for="t in themes" :key="t.id" class="glaze-slot">
        <article
          class="glaze-slab"
          :class="{ hero: openId === t.id, gone: openId && openId !== t.id }"
          :style="slabVars(t)"
          :data-id="t.id"
          :data-flip-id="t.id"
          :role="openId === t.id ? undefined : 'button'"
          :tabindex="openId && openId !== t.id ? -1 : 0"
          :aria-expanded="openId === t.id"
          :aria-hidden="openId && openId !== t.id ? true : undefined"
          :aria-label="t.name"
          @click="onSlabClick(t.id)"
          @keydown="onSlabKey($event, t.id)"
        >
          <span class="glaze-mark">{{ t.mark }}</span>
          <span class="glaze-name">{{ t.name }}</span>
          <div class="glaze-blurb">
            <p v-for="(line, i) in t.blurb" :key="i">{{ line }}</p>
          </div>
          <span class="glaze-ramp" aria-hidden="true">
            <i data-kind="empty" />
            <i data-kind="lit" data-level="1" />
            <i data-kind="lit" data-level="2" />
            <i data-kind="lit" data-level="3" />
            <i data-kind="lit" data-level="4" />
          </span>

          <div v-if="openId === t.id && revealMock" class="glaze-expand">
            <div class="glaze-mock" aria-hidden="true">
              <div class="glaze-mock-chrome">
                <h1>whereToken</h1>
                <p class="glaze-mock-total">248.60<em>M</em></p>
                <div class="damper">
                  <span class="on">合计</span>
                  <span>工具</span>
                  <span>厂家</span>
                </div>
                <div class="glaze-mock-tools">
                  <span class="when">2026-08-16 00:00</span>
                  <span class="lever">主题</span>
                  <span class="lever">刷新</span>
                </div>
              </div>
              <MockKeyboard />
            </div>
            <div class="glaze-expand-actions">
              <button type="button" class="lever glaze-back" @click.stop="backToGrid">返回釉厅</button>
              <button type="button" class="lever primary" @click.stop="commit">应用</button>
            </div>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>
