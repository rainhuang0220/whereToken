export type MockBrick = {
  kind: 'empty' | 'lit' | 'future'
  level: 0 | 1 | 2 | 3 | 4
}

export function mockBricks(weeks = 24): MockBrick[] {
  const n = weeks * 7
  const out: MockBrick[] = []
  for (let i = 0; i < n; i++) {
    if (i >= n - 5) {
      out.push({ kind: 'future', level: 0 })
      continue
    }
    const h = (i * 13 + 7) % 11
    if (h < 4) out.push({ kind: 'empty', level: 0 })
    else out.push({ kind: 'lit', level: ((h % 4) + 1) as 1 | 2 | 3 | 4 })
  }
  return out
}
