import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  heatHex,
  previewIntensity,
  publicationPending,
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
    expect(panel).toContain('应用到公开 Profile')
    expect(panel).toContain('预览公开 Profile')
    expect(panel).toContain('外观预览')
    expect(panel).not.toContain('wheretoken.theme')
  })

  it('marks a selected palette pending until the local bundle matches', () => {
    expect(publicationPending('cobalt', view({ bundle_palette: 'newsprint', status: 'pending' }))).toBe(true)
    expect(publicationPending('newsprint', view({ status: 'ready_to_publish' }))).toBe(false)
    expect(statusCopy(view({ status: 'ready_to_publish' }))).toContain('推送')
    expect(statusCopy(view({ status: 'saved_locally' }))).not.toContain('已发布到')
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
