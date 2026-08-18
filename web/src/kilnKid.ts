export const kilnKidPoses = ['scratch', 'abacus', 'toss', 'fire', 'blink', 'grin'] as const

export type KilnKidPose = (typeof kilnKidPoses)[number]

export const kilnKidMoods: Record<KilnKidPose, string> = {
  scratch: '挠头中',
  abacus: '拨珠中',
  toss: '搬煤中',
  fire: '煅烧中',
  blink: '眨眼中',
  grin: '出窑中',
}

/** 2-cell block mark. Empty quadrants are the eyes; they rotate like Kimi's moon. */
export const kilnGlyphs = ['▛▜', '▜█', '█▟', '▟█', '▙▟', '█▙', '▛█', '█▜'] as const

export function kilnGlyph(tick: number): string {
  const n = kilnGlyphs.length
  return kilnGlyphs[((tick % n) + n) % n]
}

export function kilnKidPose(tick: number): KilnKidPose {
  const n = kilnKidPoses.length
  return kilnKidPoses[((tick % n) + n) % n]
}

export function kilnKidFrame(tick: number): string {
  return kilnGlyph(tick)
}

export function kilnKidMood(tick: number): string {
  return kilnKidMoods[kilnKidPose(tick)]
}

export const kilnEyePhases = [
  { a: [1, 2], b: [5, 2] },
  { a: [2, 1], b: [5, 3] },
  { a: [4, 1], b: [5, 4] },
  { a: [5, 2], b: [5, 5] },
  { a: [5, 4], b: [2, 5] },
  { a: [4, 5], b: [1, 4] },
  { a: [1, 5], b: [1, 2] },
  { a: [1, 3], b: [2, 1] },
] as const

export function kilnEyePhase(tick: number): (typeof kilnEyePhases)[number] {
  const n = kilnEyePhases.length
  return kilnEyePhases[((tick % n) + n) % n]
}
