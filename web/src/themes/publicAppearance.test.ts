import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { resolveLang } from '../i18n'
import {
  handoffFromQuery,
  heatHex,
  previewIntensity,
  publicationPending,
  publishRunning,
  resolveVisitorPalette,
  statusCopy,
  type PublicationView,
} from './publicAppearance'

const root = dirname(fileURLToPath(import.meta.url))

function view(partial: Partial<PublicationView>): PublicationView {
  return {
    schema_version: 1,
    public_palette: 'newsprint',
    revision: '1',
    status: 'ready_to_publish',
    export_command: 'wheretoken profile build ./public-profile --public-palette newsprint',
    bundle_palette: 'newsprint',
    ...partial,
  }
}

describe('public profile appearance', () => {
  it('keeps share link, visitor choice, and the owner default in that order', () => {
    expect(resolveVisitorPalette({ url: 'magenta', visitor: 'cobalt', published: 'newsprint' })).toBe('magenta')
    expect(resolveVisitorPalette({ url: 'kiln', visitor: 'cobalt', published: 'newsprint' })).toBe('cobalt')
    expect(resolveVisitorPalette({ visitor: 'nope', published: 'cobalt' })).toBe('cobalt')
    expect(resolveVisitorPalette({ published: 'dashboard' })).toBe('newsprint')
  })

  it('does not treat a dashboard glaze as a public palette', () => {
    const page = readFileSync(join(root, '../pages/Themes.vue'), 'utf8')
    expect(page).toContain('PublicAppearance')
    expect(page).not.toContain('kiln-to-newsprint')
    const panel = readFileSync(join(root, 'PublicAppearance.vue'), 'utf8')
    const dict = readFileSync(join(root, '../i18n.ts'), 'utf8')
    expect(panel).toContain("tr('appearance.save')")
    expect(panel).toContain("tr('appearance.publish')")
    expect(panel).toContain("tr('appearance.confirm')")
    expect(panel).toContain("tr('appearance.preview')")
    expect(panel).toContain("tr('appearance.previewLabel')")
    expect(panel).toContain("chooseLang('zh-CN')")
    expect(panel).toContain('@click="confirmPublish"')
    expect(dict).toContain('Save on this computer')
    expect(dict).toContain('Publish to my GitHub profile')
    expect(dict).toContain('Confirm publish to both repositories')
    expect(dict).toContain('Preview public profile')
    expect(dict).toContain('Appearance preview')
    expect(dict).toContain('仅保存本机')
    expect(dict).toContain('发布到我的 GitHub 主页')
    expect(dict).toContain('确认发布到这两个仓库')
    expect(dict).toContain('预览公开 Profile')
    expect(dict).toContain('外观预览')
    expect(dict).not.toContain('navigator.language')
    expect(panel).not.toContain('wheretoken.theme')
  })

  it('marks a selected palette pending until the local bundle matches', () => {
    expect(publicationPending('cobalt', view({ bundle_palette: 'newsprint', status: 'pending' }))).toBe(true)
    expect(publicationPending('newsprint', view({ status: 'ready_to_publish' }))).toBe(false)
    expect(statusCopy(view({ status: 'ready_to_publish' }))).toContain('pushed')
    expect(statusCopy(view({ status: 'ready_to_publish' }))).not.toContain('推送')
    expect(statusCopy(view({ status: 'ready_to_publish' }), 'zh-CN')).toContain('推送')
    expect(statusCopy(view({ status: 'saved_locally' }))).not.toContain('已发布到')
    expect(statusCopy(view({ status: 'saved_locally' }), 'zh-CN')).not.toContain('已发布到')
  })

  it('treats a publish link as a draft until the owner confirms', () => {
    expect(handoffFromQuery({ public_palette: 'magenta', intent: 'publish' })).toEqual({ palette: 'magenta', publish: true })
    expect(handoffFromQuery({ public_palette: 'kiln', intent: 'publish' })).toEqual({ palette: null, publish: false })
    expect(handoffFromQuery({ public_palette: '../whereToken', intent: 'publish' })).toEqual({ palette: null, publish: false })
    expect(handoffFromQuery({ public_palette: 'cobalt' })).toEqual({ palette: 'cobalt', publish: false })
    expect(publishRunning('waiting_pages')).toBe(true)
    expect(publishRunning('verified')).toBe(false)
    const panel = readFileSync(join(root, 'PublicAppearance.vue'), 'utf8')
    expect(panel).toContain('@click="confirmPublish"')
  })

  it('starts in English unless the URL or a saved choice says otherwise', () => {
    const store = new Map<string, string>()
    const previous = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => { store.set(key, value) },
      },
    })
    try {
      expect(resolveLang('')).toBe('en')
      expect(resolveLang('?lang=zh-CN')).toBe('zh-CN')
      expect(resolveLang('?lang=nope')).toBe('en')
      store.set('wt-lang', 'zh-CN')
      expect(resolveLang('')).toBe('zh-CN')
      expect(resolveLang('?lang=en')).toBe('en')
      store.set('wt-lang', 'fr')
      expect(resolveLang('')).toBe('en')
    } finally {
      if (previous) Object.defineProperty(globalThis, 'localStorage', previous)
      else delete (globalThis as { localStorage?: unknown }).localStorage
    }
  })

  it('paints newsprint as neutral ink and cobalt as a distinct color', () => {
    const paper = heatHex('newsprint', 0.2)
    const ink = heatHex('newsprint', 1)
    const cobalt = heatHex('cobalt', 1)
    expect(paper).toMatch(/^#[0-9a-f]{6}$/)
    expect(ink).not.toBe(paper)
    expect(cobalt).not.toBe(ink)
    expect(previewIntensity(0)).toBeGreaterThanOrEqual(0)
    expect(previewIntensity(3)).toBeLessThanOrEqual(1)
  })
})
