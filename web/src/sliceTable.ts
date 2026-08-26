export const sliceHeads = ['未命中', '缓存读', '缓存写', '输出', '合计', '命中率']

export function rowIsSelectable(quality: string): boolean {
  return quality !== 'absent'
}

export function isRowActivateKey(key: string): boolean {
  return key === 'Enter' || key === ' '
}
