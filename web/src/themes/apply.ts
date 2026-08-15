import { STORAGE_KEY, resolveThemeId, type ThemeId } from './manifest'

export type ThemeRoot = {
  setAttribute(name: string, value: string): void
}

export type ThemeStorage = {
  setItem(key: string, value: string): void
}

export function applyTheme(
  id: string,
  opts?: {
    root?: ThemeRoot
    storage?: ThemeStorage
    persist?: boolean
  },
): ThemeId {
  const resolved = resolveThemeId(id)
  const root = opts?.root ?? (typeof document !== 'undefined' ? document.documentElement : undefined)
  const storage =
    opts?.storage ?? (typeof localStorage !== 'undefined' ? localStorage : undefined)
  root?.setAttribute('data-theme', resolved)
  if (opts?.persist !== false) {
    try {
      storage?.setItem(STORAGE_KEY, resolved)
    } catch {
      /* private mode / tests without storage */
    }
  }
  return resolved
}
