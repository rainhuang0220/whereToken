import { describe, expect, it } from 'vitest'
import {
  avatarInitial,
  displayHandle,
  firstMenuIndex,
  isUnauthorized,
  lastMenuIndex,
  menuOffset,
  moveMenuIndex,
  safeReturnPath,
  triggerMenuKey,
  UNAUTHORIZED,
  type MenuItem,
} from './account'

describe('safeReturnPath', () => {
  it('allows in-site paths only', () => {
    expect(safeReturnPath('/settings/devices')).toBe('/settings/devices')
    expect(safeReturnPath('/app')).toBe('/app')
    expect(safeReturnPath('https://evil.test')).toBe('/app')
    expect(safeReturnPath('//evil.test')).toBe('/app')
    expect(safeReturnPath('')).toBe('/app')
  })
})

describe('displayHandle', () => {
  it('prefixes @ once', () => {
    expect(displayHandle('rainhuang0220')).toBe('@rainhuang0220')
    expect(displayHandle('@rainhuang0220')).toBe('@rainhuang0220')
  })
  it('keeps a long login intact for the menu identity', () => {
    expect(displayHandle('very-long-github-username-example')).toBe(
      '@very-long-github-username-example',
    )
  })
  it('initial from login', () => {
    expect(avatarInitial('rainhuang0220')).toBe('R')
    expect(avatarInitial('')).toBe('?')
  })
})

describe('moveMenuIndex', () => {
  const items: MenuItem[] = [
    { id: 'a', kind: 'item' },
    { id: 's', kind: 'sep' },
    { id: 'b', kind: 'item' },
    { id: 'c', kind: 'item' },
  ]
  it('skips separators', () => {
    expect(moveMenuIndex(items, 0, 1)).toBe(2)
    expect(moveMenuIndex(items, 2, 1)).toBe(3)
    expect(moveMenuIndex(items, 3, 1)).toBe(0)
    expect(moveMenuIndex(items, 0, -1)).toBe(3)
  })
  it('first and last skip separators', () => {
    expect(firstMenuIndex(items)).toBe(0)
    expect(lastMenuIndex(items)).toBe(3)
  })
})

describe('triggerMenuKey', () => {
  it('opens, toggles, and escapes like a menu button', () => {
    expect(triggerMenuKey(false, 'Enter')).toBe('toggle')
    expect(triggerMenuKey(false, ' ')).toBe('toggle')
    expect(triggerMenuKey(true, 'Enter')).toBe('toggle')
    expect(triggerMenuKey(false, 'ArrowDown')).toBe('open-first')
    expect(triggerMenuKey(false, 'ArrowUp')).toBe('open-last')
    expect(triggerMenuKey(true, 'Escape')).toBe('close')
    expect(triggerMenuKey(false, 'Escape')).toBe('')
    expect(triggerMenuKey(false, 'Tab')).toBe('')
  })
})

describe('menuOffset', () => {
  it('aligns end and flips when the menu would leave the viewport', () => {
    const below = menuOffset(
      { top: 20, left: 200, width: 80, height: 32 },
      { width: 220, height: 240 },
      { width: 390, height: 844 },
    )
    expect(below.top).toBe(36)
    expect(below.left).toBe(80 - 220)
    const flip = menuOffset(
      { top: 700, left: 200, width: 80, height: 32 },
      { width: 220, height: 240 },
      { width: 390, height: 844 },
    )
    expect(flip.top).toBe(-244)
    const clamp = menuOffset(
      { top: 20, left: 10, width: 40, height: 32 },
      { width: 280, height: 200 },
      { width: 390, height: 844 },
    )
    expect(clamp.left).toBe(8 - 10)
  })
})

describe('isUnauthorized', () => {
  it('matches the session-expired sentinel', () => {
    expect(isUnauthorized(new Error(UNAUTHORIZED))).toBe(true)
    expect(isUnauthorized(new Error('无法加载数据'))).toBe(false)
  })
})
