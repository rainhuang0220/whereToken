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

/** Claude-style slab with two vertical eye bars. */
export const kilnKidFrames: Record<KilnKidPose, readonly [string, string, string]> = {
  scratch: ['▄██████▄', '█ ▌  ~ █', '▀██████▀'],
  abacus: ['▄██████▄', '█ ▌  ▌ █', '▀██████▀'],
  toss: ['▄██████▄', '█ ▌  ▌*█', '▀██████▀'],
  fire: ['▄██^^██▄', '█ ▌  ▌ █', '▀██████▀'],
  blink: ['▄██████▄', '█ ▂  ▂ █', '▀██████▀'],
  grin: ['▄██████▄', '█ ▌  ▌ █', '▀██████▀'],
}

export function kilnKidPose(tick: number): KilnKidPose {
  const n = kilnKidPoses.length
  return kilnKidPoses[((tick % n) + n) % n]
}

export function kilnKidFrame(tick: number): string {
  return kilnKidFrames[kilnKidPose(tick)].join('\n')
}

export function kilnKidMood(tick: number): string {
  return kilnKidMoods[kilnKidPose(tick)]
}
