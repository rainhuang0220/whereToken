import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const SRC = join(dirname(fileURLToPath(import.meta.url)))

describe('kiln wall chrome', () => {
  const vue = readFileSync(join(SRC, 'components/KilnWall.vue'), 'utf8')
  const css = readFileSync(join(SRC, 'styles.css'), 'utf8')

  it('does not render month headers or a weekday gutter', () => {
    expect(vue).not.toMatch(/class="months"/)
    expect(vue).not.toMatch(/class="wdays"/)
    expect(vue).not.toMatch(/monthLabels/)
    expect(vue).not.toMatch(/wdays/)
    expect(vue).not.toMatch(/['"]一['"]/)
    expect(css).not.toMatch(/\.months\b/)
    expect(css).not.toMatch(/\.wdays\b/)
  })

  it('shows a two-line pointer-following caption, not a cell-anchored dump', () => {
    expect(vue).toMatch(/brickCaption/)
    expect(vue).toMatch(/kiln-float/)
    expect(vue).toMatch(/pointer-events:\s*none/)
    expect(vue).toMatch(/clientX/)
    expect(vue).not.toMatch(/:title=/)
    expect(vue).not.toMatch(/消耗/)
    expect(vue).not.toMatch(/未命中/)
    const cssFloat = css.match(/\.kiln-float\s*\{([^}]+)\}/)
    expect(cssFloat, 'missing .kiln-float').toBeTruthy()
    expect(cssFloat![1]).toMatch(/position:\s*fixed/)
    expect(cssFloat![1]).toMatch(/pointer-events:\s*none/)
  })
})

describe('home rail copy', () => {
  it('labels the rescan control 刷新, not 再扫', () => {
    const home = readFileSync(join(SRC, 'pages/Home.vue'), 'utf8')
    expect(home).toMatch(/store\.loading \? '[^']+' : '刷新'/)
    expect(home).not.toMatch(/再扫/)
  })

  it('offers 主题 as a single control, not a row of glaze marks', () => {
    const home = readFileSync(join(SRC, 'pages/Home.vue'), 'utf8')
    expect(home).toMatch(/to="\/themes"/)
    expect(home).toMatch(/>主题</)
    expect(home).not.toMatch(/ThemeSwitcher/)
    expect(home).not.toMatch(/t\.mark/)
    expect(home).not.toMatch(/packs/)
  })
})

describe('firing observatory', () => {
  const home = readFileSync(join(SRC, 'pages/Home.vue'), 'utf8')
  const store = readFileSync(join(SRC, 'stores/summary.ts'), 'utf8')
  const api = readFileSync(join(SRC, 'api.ts'), 'utf8')
  const css = readFileSync(join(SRC, 'styles.css'), 'utf8')

  it('does not wipe summary to empty when 刷新 starts', () => {
    expect(store).not.toMatch(/this\.payload\s*=\s*null/)
    expect(store).toMatch(/if\s*\(\s*this\.loading\s*\)/)
    expect(api).toMatch(/\/api\/scan/)
    expect(home).toMatch(/class="hearth"/)
    expect(home).toMatch(/FiringVeil/)
    expect(home).toMatch(/<KilnWall/)
    const veil = readFileSync(join(SRC, 'components/FiringVeil.vue'), 'utf8')
    expect(veil).toMatch(/aria-live="polite"/)
    expect(veil).toMatch(/class="firing-veil"/)
  })

  it('keeps the kiln in the hearth and charges with theme embers, static under reduced motion', () => {
    expect(css).toMatch(/\.firing-veil/)
    expect(css).toMatch(/\.firing-charge/)
    expect(css).toMatch(/--ember-/)
    expect(css).toMatch(/prefers-reduced-motion:\s*reduce/)
    const reduced = css.match(/@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{([\s\S]+?)\n\}/)
    expect(reduced, 'missing reduced-motion block').toBeTruthy()
    expect(reduced![1]).toMatch(/\.firing-charge/)
    expect(home).toMatch(/KilnWall/)
  })
})
