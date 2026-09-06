import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  loginCta,
  loginHeadline,
  loginNoRepo,
  loginPrivacy,
  neverSyncedItems,
  neverSyncedTitle,
  oauthErrorBody,
  oauthErrorTitle,
  pairConfirm,
  pairOpenDash,
  pairSuccessTitle,
  pairTitle,
  syncedItems,
  waitingSyncBody,
} from './copy'

function page(name: string) {
  return readFileSync(new URL(`../pages/${name}`, import.meta.url), 'utf8')
}

function home() {
  return readFileSync(new URL('../pages/Home.vue', import.meta.url), 'utf8')
}

describe('hosted copy', () => {
  it('keeps login, error, onboarding and pair phrases', () => {
    expect(loginHeadline).toMatch(/机器/)
    expect(loginCta).toBe('使用 GitHub 登录')
    expect(loginPrivacy).toMatch(/GitHub/)
    expect(loginNoRepo).toMatch(/仓库/)
    expect(oauthErrorTitle).toBe('登录没有完成')
    expect(oauthErrorBody).toMatch(/重新尝试/)
    expect(neverSyncedTitle).toBe('尚未同步用量')
    expect(waitingSyncBody).toMatch(/首次同步/)
    expect(pairTitle).toMatch(/设备/)
    expect(pairSuccessTitle).toMatch(/已连接/)
    expect(pairConfirm).toBe('连接设备')
    expect(pairOpenDash).toBe('打开仪表盘')
    expect(syncedItems).toContain('token counts')
    expect(syncedItems).toContain('source health')
    expect(neverSyncedItems).toContain('prompts')
    expect(neverSyncedItems).toContain('conversations')
    expect(neverSyncedItems).toContain('raw databases')
  })
})

describe('hosted pages', () => {
  it('login has brand, CTA, privacy, preview and oauth error state', () => {
    const src = page('Login.vue')
    expect(src).toContain('loginHeadline')
    expect(src).toContain('loginCta')
    expect(src).toContain('oauthFailed')
    expect(src).toContain('oauthErrorTitle')
    expect(src).toContain('syncWhatTitle')
    expect(src).toContain('siteLabel')
    expect(src).toContain('class="forge"')
    expect(src).toContain('class="ext"')
    expect(src).not.toContain('Hosted')
    expect(src).not.toContain('What gets synced?')
    expect(src).not.toContain('Project Site')
    expect(src).not.toMatch(/Features|Testimonials|Newsletter/)
  })

  it('pair has pending card and success state', () => {
    const src = page('Pair.vue')
    expect(src).toContain('pairTitle')
    expect(src).toContain('pairSuccessTitle')
    expect(src).toContain('pairConfirm')
    expect(src).toContain('pairOpenDash')
    expect(src).toContain('/api/v1/pair/challenge')
    expect(src).not.toContain('Connect device')
    expect(src).not.toContain('Open Dashboard')
  })

  it('settings covers account appearance devices privacy and danger confirm', () => {
    const src = page('Settings.vue')
    expect(src).toContain('HostedAccountMenu')
    expect(src).toContain('断开设备')
    expect(src).toContain('删除已同步数据')
    expect(src).toContain('删除账号')
    expect(src).toContain('syncedItems')
    expect(src).toContain('wheretoken sync')
    expect(src).toContain('pack.name')
    expect(src).toContain('最近在线')
    expect(src).toContain('最近同步')
    expect(src).toContain('退出只结束这个浏览器会话')
    expect(src).toContain('不等于退出 Web')
    expect(src).not.toContain('Sync now')
    expect(src).not.toContain('pack.label')
    expect(src).not.toContain('Auto Sync')
    expect(src).toContain('lever danger')
  })

  it('home hosted wait states reuse kiln empty layout and skip local-ledger copy', () => {
    const src = home()
    expect(src).toContain('neverSyncedTitle')
    expect(src).toContain('waitingSyncBody')
    expect(src).toContain('wheretoken login')
    expect(src).toContain('hosted || !payload')
    expect(src).toContain('KpiRow')
    expect(src).toContain('class="forge"')
    expect(src).not.toContain('HostedShell')
    expect(src).toContain('HostedAccountMenu')
    expect(src).toContain('未连接设备')
    expect(src).not.toContain('to="/settings/privacy"')
    expect(src).not.toContain('>设备<')
    expect(src).not.toContain('>隐私<')
    expect(src).not.toContain('>退出<')
  })

  it('themes keep local gallery and only add hosted account chrome behind isHosted', () => {
    const src = page('Themes.vue')
    expect(src).toContain('HostedAccountMenu')
    expect(src).toContain('isHosted')
    expect(src).toContain('v-if="hosted"')
  })
})

describe('hosted account menu source', () => {
  it('is a real menu button with fallback and keyboard hooks', () => {
    const src = readFileSync(new URL('../components/HostedAccountMenu.vue', import.meta.url), 'utf8')
    expect(src).toContain('role="menu"')
    expect(src).toContain('aria-haspopup="menu"')
    expect(src).toContain('acct-fallback')
    expect(src).toContain('acct-name')
    expect(src).toContain('退出登录')
    expect(src).toContain('triggerMenuKey')
    expect(src).toContain('loginCta')
    expect(src).toContain('is-skel')
    expect(src).toContain('width="24"')
    expect(src).toContain('height="24"')
    expect(src).not.toContain('outline: none')
    const css = readFileSync(new URL('../styles.css', import.meta.url), 'utf8')
    expect(css).toContain('.acct-avatar')
    expect(css).toMatch(/\.acct-avatar[\s\S]*object-fit:\s*cover/)
    expect(css).toMatch(/\.acct-name[\s\S]*text-overflow:\s*ellipsis/)
  })
})

describe('copy command', () => {
  it('shows copied and failure without alert', () => {
    const src = readFileSync(new URL('../components/CopyCommand.vue', import.meta.url), 'utf8')
    expect(src).toContain('已复制')
    expect(src).toContain('复制失败')
    expect(src).toContain('aria-live')
    expect(src).not.toContain('alert(')
  })
})
