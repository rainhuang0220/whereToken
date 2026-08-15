import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import {
  DEFAULT_THEME,
  REQUIRED_TOKENS,
  STORAGE_KEY,
  THEME_IDS,
  applyTheme,
  resolveThemeId,
  themeStylesheet,
  themes,
} from './index'

const SRC_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')

const PAINT_BITS = [
  '#070504',
  '#3a2c22',
  '#120c09',
  '#4a2714',
  '#9a3a0d',
  '#e85a10',
  '#ffc44a',
  '#e8dcc8',
  '#8a7a68',
  '#c47a3a',
  '#ff6b3d',
  'rgba(232, 90, 16',
  'rgba(74, 39, 20',
  'rgba(196, 122, 58',
  'rgba(255, 196, 74',
  'rgba(255, 180, 80',
  'rgba(255, 220, 160',
]

function walk(dir: string, acc: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    if (name === 'themes') continue
    const p = join(dir, name)
    if (statSync(p).isDirectory()) walk(p, acc)
    else if (/\.(vue|css|ts)$/.test(name)) acc.push(p)
  }
  return acc
}

function relLum(hex: string): number {
  const n = hex.replace('#', '')
  const to = (i: number) => {
    const c = parseInt(n.slice(i, i + 2), 16) / 255
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  }
  return 0.2126 * to(0) + 0.7152 * to(2) + 0.0722 * to(4)
}

function contrast(a: string, b: string): number {
  const l1 = relLum(a)
  const l2 = relLum(b)
  const hi = Math.max(l1, l2)
  const lo = Math.min(l1, l2)
  return (hi + 0.05) / (lo + 0.05)
}

describe('theme pack manifest', () => {
  it('ships kiln, moss, porcelain, jiang, qingmo, frost, day, ink', () => {
    expect([...THEME_IDS]).toEqual([
      'kiln',
      'moss',
      'porcelain',
      'jiang',
      'qingmo',
      'frost',
      'day',
      'ink',
    ])
    expect(themes.map((t) => t.id)).toEqual([...THEME_IDS])
    expect(themes.map((t) => t.mark)).toEqual(['窑', '苔', '瓷', '绛', '青', '霜', '昼', '墨'])
  })

  it('defines every required token key on every theme', () => {
    for (const theme of themes) {
      for (const key of REQUIRED_TOKENS) {
        const value = theme.tokens[key]
        expect(value, `${theme.id} missing ${key}`).toEqual(expect.any(String))
        expect(value.length, `${theme.id}.${key} empty`).toBeGreaterThan(0)
      }
    }
  })

  it('snapshots required CSS variable names', () => {
    expect([...REQUIRED_TOKENS]).toMatchInlineSnapshot(`
      [
        "void",
        "clay",
        "mortar",
        "ember-1",
        "ember-2",
        "ember-3",
        "ember-4",
        "bone",
        "ash",
        "copper",
        "warn",
        "glow",
        "hi",
        "lo",
        "scheme",
      ]
    `)
  })

  it('defaults to kiln', () => {
    expect(DEFAULT_THEME).toBe('kiln')
    expect(resolveThemeId(null)).toBe('kiln')
    expect(resolveThemeId('')).toBe('kiln')
    expect(resolveThemeId('nope')).toBe('kiln')
    expect(resolveThemeId('frost')).toBe('frost')
    expect(resolveThemeId('day')).toBe('day')
    expect(resolveThemeId('ink')).toBe('ink')
  })

  it('persists selection under wheretoken.theme and sets data-theme', () => {
    expect(STORAGE_KEY).toBe('wheretoken.theme')
    const attrs: Record<string, string> = {}
    const store: Record<string, string> = {}
    applyTheme('qingmo', {
      root: {
        setAttribute(name, value) {
          attrs[name] = value
        },
      },
      storage: {
        setItem(key, value) {
          store[key] = value
        },
      },
    })
    expect(attrs['data-theme']).toBe('qingmo')
    expect(store[STORAGE_KEY]).toBe('qingmo')
  })

  it('can preview a glaze without writing localStorage', () => {
    const attrs: Record<string, string> = {}
    const store: Record<string, string> = {}
    applyTheme('ink', {
      persist: false,
      root: {
        setAttribute(name, value) {
          attrs[name] = value
        },
      },
      storage: {
        setItem(key, value) {
          store[key] = value
        },
      },
    })
    expect(attrs['data-theme']).toBe('ink')
    expect(store[STORAGE_KEY]).toBeUndefined()
  })

  it('keeps body text, ash labels, and emphasis readable on the field', () => {
    for (const theme of themes) {
      const { void: bg, bone, ash, 'ember-4': hot, warn } = theme.tokens
      expect(contrast(bone, bg), `${theme.id} bone/void`).toBeGreaterThanOrEqual(4.5)
      expect(contrast(ash, bg), `${theme.id} ash/void`).toBeGreaterThanOrEqual(4.5)
      expect(contrast(hot, bg), `${theme.id} ember-4/void`).toBeGreaterThanOrEqual(4.5)
      expect(contrast(warn, bg), `${theme.id} warn/void`).toBeGreaterThanOrEqual(4.5)
    }
  })

  it('keeps empty kiln cells visible against mortar and the page field', () => {
    for (const theme of themes) {
      const { void: bg, clay, mortar } = theme.tokens
      expect(clay, `${theme.id} clay==void`).not.toBe(bg)
      expect(clay, `${theme.id} clay==mortar`).not.toBe(mortar)
      expect(contrast(clay, bg), `${theme.id} clay/void`).toBeGreaterThanOrEqual(1.15)
      expect(contrast(clay, mortar), `${theme.id} clay/mortar`).toBeGreaterThanOrEqual(1.15)
    }
  })

  it('keeps 墨 ink grayscale', () => {
    const ink = themes.find((t) => t.id === 'ink')
    expect(ink, 'missing ink').toBeTruthy()
    expect(ink!.tokens.scheme).toBe('light')
    for (const key of REQUIRED_TOKENS) {
      if (key === 'scheme') continue
      const value = ink!.tokens[key]
      expect(value, key).toMatch(/^#[0-9a-fA-F]{6}$/)
      expect(value.slice(1, 3), `${key} chroma`).toBe(value.slice(3, 5))
      expect(value.slice(3, 5), `${key} chroma`).toBe(value.slice(5, 7))
    }
  })

  it('lets the html boot script paint every pack before modules load', () => {
    const html = readFileSync(join(SRC_ROOT, '..', 'index.html'), 'utf8')
    const match = html.match(/var allowed = \[([^\]]+)\]/)
    expect(match, 'missing allowed list').toBeTruthy()
    const allowed = [...match![1].matchAll(/'([^']+)'/g)].map((m) => m[1])
    expect(allowed).toEqual([...THEME_IDS])
  })

  it('emits [data-theme] CSS so bricks recolor from variables', () => {
    const css = themeStylesheet()
    expect(css).toContain(':root,[data-theme="kiln"]')
    for (const theme of themes) {
      expect(css).toContain(`[data-theme="${theme.id}"]`)
      for (const key of REQUIRED_TOKENS) {
        if (key === 'scheme') {
          expect(css).toContain(`color-scheme:${theme.tokens.scheme}`)
          continue
        }
        expect(css).toContain(`--${key}:${theme.tokens[key]}`)
      }
    }
  })
})

describe('no leftover kiln paint outside the pack', () => {
  it('does not keep a 消耗 watermark in Vue', () => {
    const app = readFileSync(join(SRC_ROOT, 'App.vue'), 'utf8')
    expect(app).not.toMatch(/class="watermark"/)
    expect(app).not.toMatch(/<div class="watermark"/)
    const css = readFileSync(join(SRC_ROOT, 'styles.css'), 'utf8')
    expect(css).not.toMatch(/\.watermark\b/)
  })

  it('does not hardcode kiln hex in Vue or global CSS', () => {
    const files = walk(SRC_ROOT)
    for (const file of files) {
      const text = readFileSync(file, 'utf8')
      const lower = text.toLowerCase()
      for (const bit of PAINT_BITS) {
        expect(lower, `${file} still paints ${bit}`).not.toContain(bit.toLowerCase())
      }
    }
  })
})

function ruleBody(css: string, selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = css.match(new RegExp(`${escaped}\\s*\\{([^}]+)\\}`))
  expect(match, `missing ${selector} rule`).toBeTruthy()
  return match![1]
}

describe('theme is a page, not a home strip', () => {
  it('does not ship the eight-mark rail switcher', () => {
    expect(existsSync(join(SRC_ROOT, 'themes/Switcher.vue'))).toBe(false)
    const css = readFileSync(join(SRC_ROOT, 'styles.css'), 'utf8')
    expect(css).not.toMatch(/\.packs\b/)
  })

  it('routes /themes to a glaze hall', () => {
    const router = readFileSync(join(SRC_ROOT, 'router.ts'), 'utf8')
    expect(router).toMatch(/path:\s*['"]\/themes['"]/)
    expect(router).toMatch(/createWebHistory/)
  })

  it('previews every glaze ramp and commits with 应用', () => {
    const page = readFileSync(join(SRC_ROOT, 'pages/Themes.vue'), 'utf8')
    expect(page).toMatch(/v-for="t in themes"/)
    expect(page).toMatch(/ember-1/)
    expect(page).toMatch(/ember-4/)
    expect(page).toMatch(/应用/)
    expect(page).toMatch(/返回/)
    expect(page).toMatch(/persist:\s*false/)
    expect(page).toMatch(/applyTheme\(/)
    expect(page).toMatch(/push\(\s*['"]\/['"]\s*\)/)
    for (const mark of ['窑', '苔', '瓷', '绛', '青', '霜', '昼', '墨']) {
      expect(themes.some((t) => t.mark === mark), mark).toBe(true)
    }
  })
})

describe('flat GitHub-like kiln wall', () => {
  const css = readFileSync(join(SRC_ROOT, 'styles.css'), 'utf8')

  it('does not use 3D shadows, glow, glass blend, or tile pop animation', () => {
    expect(css).not.toMatch(/box-shadow\s*:/)
    expect(css).not.toMatch(/translateY\s*\(/)
    expect(css).not.toMatch(/mix-blend-mode/)
    expect(css).not.toMatch(/@keyframes\s+fire-in/)
    expect(css).not.toMatch(/radial-gradient/)
  })

  it('does not wash panels with transparent color-mix fog outside selection', () => {
    const stripped = css.replace(/::selection\s*\{[^}]+\}/g, '')
    expect(stripped).not.toMatch(/color-mix\([^;{}]+transparent/)
  })

  it('renders bricks as tiny-radius flat fills with a clay/mortar hairline', () => {
    const brick = ruleBody(css, '.brick')
    expect(brick).toMatch(/border-radius:\s*2px/)
    expect(brick).toMatch(/background:\s*var\(--clay\)/)
    expect(brick).toMatch(/border:\s*1px solid var\(--(?:clay|mortar)\)/)
    expect(brick).not.toMatch(/animation\s*:/)
    expect(brick).not.toMatch(/transform\s*:/)
  })

  it('keeps empty days as a visible clay fill, not a transparent hole', () => {
    const empty = ruleBody(css, ".brick[data-kind='empty']")
    expect(empty).toMatch(/background:\s*var\(--clay\)/)
    expect(empty).not.toMatch(/background:\s*transparent/)
    const future = ruleBody(css, ".brick[data-kind='future']")
    expect(future).not.toMatch(/background:\s*transparent/)
  })

  it('marks the peak day with a 2px flat outline instead of a halo', () => {
    const peak = ruleBody(css, '.brick.peak')
    expect(peak).toMatch(/outline:\s*2px solid var\(--ember-4\)/)
    expect(peak).not.toMatch(/box-shadow/)
  })

  it('does not lift bricks on hover', () => {
    const hover = ruleBody(css, '.brick:hover')
    expect(hover).not.toMatch(/translateY/)
    expect(hover).not.toMatch(/transform/)
  })
})
