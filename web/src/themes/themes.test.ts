import { readFileSync, readdirSync, statSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import {
  DEFAULT_THEME,
  REQUIRED_TOKENS,
  STORAGE_KEY,
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
  it('ships kiln, moss, porcelain, jiang, qingmo, frost', () => {
    expect(themes.map((t) => t.id)).toEqual([
      'kiln',
      'moss',
      'porcelain',
      'jiang',
      'qingmo',
      'frost',
    ])
    expect(themes.map((t) => t.mark)).toEqual(['窑', '苔', '瓷', '绛', '青', '霜'])
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
      expect(contrast(clay, mortar), `${theme.id} clay/mortar`).toBeGreaterThanOrEqual(1.15)
    }
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
