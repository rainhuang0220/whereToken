import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const SRC = join(dirname(fileURLToPath(import.meta.url)))

describe('local-first dashboard', () => {
  it('does not phone Google Fonts when the observatory opens', () => {
    const html = readFileSync(join(SRC, '..', 'index.html'), 'utf8')
    expect(html).not.toMatch(/fonts\.googleapis\.com/)
    expect(html).not.toMatch(/fonts\.gstatic\.com/)
  })
})

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

  it('is one tab stop, then arrows, not 371 buttons in a picture', () => {
    const grid = readFileSync(join(SRC, 'grid.ts'), 'utf8')
    expect(vue).not.toMatch(/role="img"/)
    expect(vue).toMatch(/role="grid"/)
    expect(vue).toMatch(/tabindex/)
    expect(vue).toMatch(/kilnStep/)
    expect(grid).toMatch(/ArrowRight/)
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

describe('axis damper copy', () => {
  it('does not call the filter row 消耗', () => {
    const damper = readFileSync(join(SRC, 'components/AxisDamper.vue'), 'utf8')
    expect(damper).not.toMatch(/消耗/)
    expect(damper).toMatch(/aria-label="窑墙轴"/)
  })
})

describe('drill sessions', () => {
  it('groups session request counts like the other tables', () => {
    const vue = readFileSync(join(SRC, 'components/DrillPanel.vue'), 'utf8')
    expect(vue).toMatch(/formatCount\(row\.requests\)/)
    expect(vue).not.toMatch(/\{\{\s*row\.requests\s*\}\}/)
  })

  it('prints server costCaption and never invents $0', () => {
    const vue = readFileSync(join(SRC, 'components/DrillPanel.vue'), 'utf8')
    expect(vue).toMatch(/costCaption\(row\)/)
    expect(vue).not.toMatch(/\$0/)
    const sessionBlock = vue.slice(vue.indexOf('按会话'))
    expect(sessionBlock).toMatch(/估价/)
    expect(sessionBlock).toMatch(/costCaption\(row\)/)
  })
})

describe('cross table', () => {
  it('shows 缓存写 so 合计 matches the visible columns', () => {
    const home = readFileSync(join(SRC, 'pages/Home.vue'), 'utf8')
    expect(home).toMatch(/缓存写/)
    expect(home).toMatch(/cache_create_m/)
  })

  it('prints 估价 on the tool × vendor table without inventing $0', () => {
    const home = readFileSync(join(SRC, 'pages/Home.vue'), 'utf8')
    const cross = home.slice(home.indexOf('工具 × 厂家'))
    expect(cross).toMatch(/估价/)
    expect(cross).toMatch(/costCaption\(row\)/)
    expect(cross).not.toMatch(/\$0/)
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
    expect(home).toMatch(/KilnKid/)
    expect(home).toMatch(/rail-brand/)
    expect(home).toMatch(/status-line/)
    expect(home).toMatch(/whisper/)
    expect(home).toMatch(/cold-kiln/)
    expect(home).toMatch(/kiln-mouth/)
    const veil = readFileSync(join(SRC, 'components/FiringVeil.vue'), 'utf8')
    expect(veil).toMatch(/aria-live="polite"/)
    expect(veil).toMatch(/class="firing-veil"/)
    expect(veil).toMatch(/KilnKid/)
    expect(veil).toMatch(/firing-mood/)
  })

  it('keeps the kiln in the hearth and charges with theme embers, static under reduced motion', () => {
    expect(css).toMatch(/\.firing-veil/)
    expect(css).toMatch(/\.firing-charge/)
    expect(css).toMatch(/\.kiln-kid/)
    expect(css).toMatch(/--ember-/)
    expect(css).toMatch(/prefers-reduced-motion:\s*reduce/)
    const reduced = css.match(/@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{([\s\S]+?)\n\}/)
    expect(reduced, 'missing reduced-motion block').toBeTruthy()
    expect(reduced![1]).toMatch(/\.firing-charge/)
    expect(reduced![1]).toMatch(/kiln-kid/)
    expect(home).toMatch(/KilnWall/)
  })
})
