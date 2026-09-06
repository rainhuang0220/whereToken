import { describe, expect, it } from 'vitest'
import { avatarInitial, displayHandle, moveMenuIndex, safeReturnPath, type MenuItem } from './account'

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
})
