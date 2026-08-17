export const kilnKidPoses = ['scratch', 'abacus', 'toss', 'fire', 'blink', 'grin'] as const

export type KilnKidPose = (typeof kilnKidPoses)[number]

export const kilnKidMoods: Record<KilnKidPose, string> = {
  scratch: '挠头',
  abacus: '拨算盘',
  toss: '投煤',
  fire: '煅烧',
  blink: '眨眼',
  grin: '出窑',
}

export const kilnKidFrames: Record<KilnKidPose, readonly [string, string, string, string]> = {
  scratch: ['  ∩∩~  ', ' (•ᴗ•) ', ' /|~|\\ ', '  ∪∪   '],
  abacus: ['  ∩∩   ', ' (•ᴗ•) ', ' /|≡|\\ ', '  ∪∪   '],
  toss: ['  ∩∩*  ', ' (•ᴗ•) ', ' /| *\\ ', '  ∪∪   '],
  fire: ['  ∩∩   ', ' (✧ᴗ✧) ', ' /|∩|\\ ', '  ∪∪   '],
  blink: ['  ∩∩   ', ' (•-•) ', ' /|  \\ ', '  ∪∪   '],
  grin: ['  ∩∩   ', ' (✧ᴗ✧) ', ' /|  \\ ', '  ∪∪   '],
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

export const kilnTips = [
  '煤要一块一块加',
  '算盘拨一下，账就清一点',
  '空窑也要守着',
  '命中率不含输出',
  'Cursor / Trae 的云账要等一会儿',
] as const

export function kilnTipAt(tick: number): string {
  const n = kilnTips.length
  return kilnTips[((tick % n) + n) % n]
}
