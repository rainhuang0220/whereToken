import { renderToString } from '@vue/server-renderer'
import { createSSRApp, type Component } from 'vue'
import { describe, expect, it } from 'vitest'
import EstimateModal from './components/EstimateModal.vue'
import KpiRow from './components/KpiRow.vue'
import { formatCost2 } from './format'
import type { ModelView, SliceView, UsagePortrait } from './types'

const all: SliceView = {
  id: 'all',
  label: '合计',
  miss: 100_000_000,
  cache_read: 200_000_000,
  cache_create: 10_000_000,
  output: 7_360_000,
  total: 317_360_000,
  miss_m: '100.00 M',
  cache_read_m: '200.00 M',
  cache_create_m: '10.00 M',
  output_m: '7.36 M',
  total_m: '317.36 M',
  hit_rate: 66.7,
  hit_rate_text: '66.7%',
  requests: 52_927,
  user_turns: 12_123,
  quality: 'authoritative',
  cost_status: 'complete',
  cost_usd: '$2291.4117',
}

function model(over: Partial<ModelView>): ModelView {
  return {
    id: 'claude-sonnet-4.6',
    label: 'claude-sonnet-4.6',
    miss: 50_000_000,
    cache_read: 120_000_000,
    cache_create: 5_000_000,
    output: 4_000_000,
    total: 179_000_000,
    miss_m: '50.00 M',
    cache_read_m: '120.00 M',
    cache_create_m: '5.00 M',
    output_m: '4.00 M',
    total_m: '179.00 M',
    hit_rate: 70.5,
    hit_rate_text: '70.5%',
    requests: 30_000,
    user_turns: 8_000,
    quality: 'authoritative',
    cost_status: 'complete',
    cost_usd: '$191.2001',
    vendor: 'anthropic',
    unit_prices: { miss: 5, cache_read: 0.5, cache_create: 6.25, output: 25 },
    ...over,
  }
}

const models: ModelView[] = [
  model({}),
  model({
    id: 'glm-5.2',
    label: 'glm-5.2',
    vendor: 'zhipu',
    total: 60_000_000,
    total_m: '60.00 M',
    cost_status: 'partial',
    cost_usd: '$95.5000',
    unit_prices: { miss: 1, cache_read: 0.2, output: 3.2 },
  }),
]

const vendors: SliceView[] = [
  { ...all, id: 'anthropic', label: 'Anthropic' },
  { ...all, id: 'zhipu', label: '智谱' },
]

function render(root: Component, props: Record<string, unknown>): Promise<string> {
  return renderToString(createSSRApp(root, props))
}

const fullProps = {
  all,
  maxStreak: 12,
  currentStreak: 3,
  todayTotalM: '4.21 M',
  peakTotalM: '18.40 M',
  peakDate: '2026-08-11',
  portrait: {
    state: 'ok',
    primary: '重度烧窑',
    tags: ['多模型探索', '节奏稳定'],
    detail: '本周期 317.36 M tokens\n活跃 45 天',
  } satisfies UsagePortrait,
  byModel: models,
  byVendor: vendors,
}

describe('KpiRow', () => {
  it('renders exactly the ten v0.6.0 labels in grid order', async () => {
    const html = await render(KpiRow, fullProps)
    const labels = [
      '总用量',
      '命中率',
      '最长连烧',
      '当日用量',
      '估价',
      '当前连烧',
      '请求',
      '用户回合',
      '单日最高',
      '用户画像',
    ]
    let at = -1
    for (const label of labels) {
      const i = html.indexOf(label)
      expect(i, label).toBeGreaterThan(at)
      at = i
    }
  })

  it('formats the estimate 2-decimals and wires today / peak cells', async () => {
    const html = await render(KpiRow, fullProps)
    expect(html).toContain('$2,291.41')
    expect(html).toContain('4.21 M')
    expect(html).toContain('18.40 M')
    expect(html).toContain('2026-08-11')
    expect(html).toContain('当前周期总估价 · API 等价估算，非实际账单')
    // The KPI cell is a button so Enter/Space open the detail modal.
    expect(html).toMatch(/<button[^>]*class="[^"]*read-cell read-btn/)
  })

  it('shows — for today and peak when the whole window has no data', async () => {
    const empty: SliceView = {
      ...all,
      total: 0,
      requests: 0,
      user_turns: 0,
      quality: '',
      hit_rate: null,
      hit_rate_text: '—',
      cost_status: undefined,
      cost_usd: undefined,
    }
    const html = await render(KpiRow, {
      ...fullProps,
      all: empty,
      todayTotalM: '0.00 M',
      peakTotalM: '0.00 M',
      peakDate: '',
      portrait: { state: 'none', primary: '—' },
    })
    expect(html).not.toContain('0.00 M')
    expect(html).not.toContain('$0')
  })

  it('renders the portrait states: none, insufficient, ok', async () => {
    const none = await render(KpiRow, { ...fullProps, portrait: { state: 'none', primary: '—' } })
    expect(none).toMatch(/用户画像<\/span><strong>—<\/strong>/)

    const ins = await render(KpiRow, {
      ...fullProps,
      portrait: { state: 'insufficient', primary: '数据不足' },
    })
    expect(ins).toMatch(/用户画像<\/span><strong>数据不足<\/strong>/)

    const ok = await render(KpiRow, fullProps)
    expect(ok).toMatch(/用户画像<\/span><strong>重度烧窑<\/strong>/)
    expect(ok).toContain('多模型探索 · 节奏稳定')
    expect(ok).toContain('本周期 317.36 M tokens')
  })
})

describe('EstimateModal', () => {
  it('lists model rows under vendor headers; TOTAL matches the all-slice estimate', async () => {
    const total = formatCost2(all.cost_usd)
    const html = await render(EstimateModal, { models, vendors, total })
    expect(html).toContain('role="dialog"')
    expect(html).toContain('aria-modal="true"')
    expect(html).toContain('Anthropic')
    expect(html).toContain('智谱')
    expect(html).toContain('claude-sonnet-4.6')
    expect(html).toContain('179.00 M')
    expect(html).toContain('in $5.00 · 缓存读 $0.50 · 缓存写 $6.25 · 出 $25.00')
    expect(html).toContain('$191.2001')
    // Partial rows keep the honesty suffix, never a rounded-up total.
    expect(html).toContain('$95.5000 · 部分')
    expect(html).toContain('合计')
    expect(html).toContain(total)
    expect(total).toBe('$2,291.41')
    expect(html).toContain('API 等价估算，非实际账单')
  })

  it('marks unpriced models with — and a 部分用量无价 note, never $0', async () => {
    const unpriced = model({
      id: '',
      label: '(未知模型)',
      vendor: 'mystery',
      cost_status: 'unavailable',
      cost_usd: undefined,
      unit_prices: {},
    })
    const html = await render(EstimateModal, { models: [unpriced], vendors: [], total: '—' })
    expect(html).toContain('(未知模型)')
    expect(html).toContain('部分用量无价')
    expect(html).not.toContain('$0')
  })
})
