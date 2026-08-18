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

/** Copied from MoonshotAI/kimi-code welcome.ts */
export const kimiLogo = ['▐█▛█▛█▌', '▐█████▌'] as const

/** Copied from MoonshotAI/kimi-code rendering.ts MOON_SPINNER_FRAMES */
export const moonGlyphs = ['🌑', '🌒', '🌓', '🌔', '🌕', '🌖', '🌗', '🌘'] as const

export function moonGlyph(tick: number): string {
  const n = moonGlyphs.length
  return moonGlyphs[((tick % n) + n) % n]
}

export function kilnKidPose(tick: number): KilnKidPose {
  const n = kilnKidPoses.length
  return kilnKidPoses[((tick % n) + n) % n]
}

export function kilnKidFrame(tick: number): string {
  return kimiLogo.join('\n')
}

export function kilnKidMood(tick: number): string {
  return kilnKidMoods[kilnKidPose(tick)]
}
