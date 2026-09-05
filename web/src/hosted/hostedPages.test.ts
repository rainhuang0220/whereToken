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
  onboardTitle,
  pairSuccessTitle,
  pairTitle,
  syncedItems,
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
    expect(onboardTitle).toMatch(/设备/)
    expect(neverSyncedTitle).toMatch(/同步/)
    expect(pairTitle).toMatch(/设备/)
    expect(pairSuccessTitle).toMatch(/已连接/)
    expect(syncedItems).toContain('token counts')
    expect(neverSyncedItems).toContain('prompts')
  })
})

describe('hosted pages', () => {
  it('login has brand, CTA, privacy, preview and oauth error state', () => {
    const src = page('Login.vue')
    expect(src).toContain('loginHeadline')
    expect(src).toContain('loginCta')
    expect(src).toContain('oauthFailed')
    expect(src).toContain('oauthErrorTitle')
    expect(src).toContain('Preview')
    expect(src).toContain('What gets synced?')
    expect(src).toContain('Project Site')
    expect(src).not.toMatch(/Features|Testimonials|Newsletter/)
  })

  it('pair has pending card and success state', () => {
    const src = page('Pair.vue')
    expect(src).toContain('pairTitle')
    expect(src).toContain('pairSuccessTitle')
    expect(src).toContain('Connect device')
    expect(src).toContain('Open Dashboard')
    expect(src).toContain('/api/v1/pair/challenge')
  })

  it('devices lists last seen/sync and revoke', () => {
    const src = page('Devices.vue')
    expect(src).toContain('Last seen')
    expect(src).toContain('Last sync')
    expect(src).toContain('Revoke')
    expect(src).toContain('wheretoken login')
  })

  it('privacy splits synced / never / danger zone', () => {
    const src = page('Privacy.vue')
    expect(src).toContain('syncedItems')
    expect(src).toContain('neverSyncedItems')
    expect(src).toContain('Account controls')
    expect(src).toContain('/api/v1/account/usage')
  })

  it('home hosted onboarding and status row', () => {
    const src = home()
    expect(src).toContain('neverSyncedTitle')
    expect(src).toContain('wheretoken sync')
    expect(src).toContain('hosted-status')
    expect(src).toContain('HostedShell')
  })
})
