export function rowIsSelectable(quality: string): boolean {
  return quality !== 'absent'
}

export function isRowActivateKey(key: string): boolean {
  return key === 'Enter' || key === ' '
}
