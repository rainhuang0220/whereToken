export const PUBLIC_PALETTES = ['cobalt', 'magenta', 'newsprint'] as const

export type PublicPaletteId = (typeof PUBLIC_PALETTES)[number]

export type PublicationStatus =
  | 'unconfigured'
  | 'saved_locally'
  | 'pending'
  | 'ready_to_publish'
  | 'failed'
  | 'hosted'
  | 'loading'

export interface PublicationView {
  schema_version: number
  public_palette: PublicPaletteId
  revision: string
  updated_at?: string
  bundle_palette?: string
  bundle_revision?: string
  status: PublicationStatus
  preview_path?: string
  export_command: string
  error?: string
}

type Stop = [number, [number, number, number]]

const STOPS: Record<PublicPaletteId, Stop[]> = {
  cobalt: [
    [0, [0.88, 0.055, 260]],
    [0.25, [0.72, 0.14, 260]],
    [0.6, [0.54, 0.17, 260]],
    [1, [0.46, 0.18, 260]],
  ],
  magenta: [
    [0, [0.88, 0.08, 340]],
    [0.25, [0.72, 0.18, 340]],
    [0.6, [0.54, 0.18, 340]],
    [1, [0.46, 0.19, 340]],
  ],
  newsprint: [
    [0, [0.86, 0, 0]],
    [0.25, [0.67, 0, 0]],
    [0.6, [0.43, 0, 0]],
    [1, [0.2, 0, 0]],
  ],
}

export const PALETTE_LABEL: Record<PublicPaletteId, string> = {
  cobalt: 'Cobalt',
  magenta: 'Magenta',
  newsprint: 'Newsprint',
}

export function isPublicPalette(id: string | undefined | null): id is PublicPaletteId {
  return id === 'cobalt' || id === 'magenta' || id === 'newsprint'
}

export function coercePalette(id: string | undefined | null): PublicPaletteId {
  return isPublicPalette(id) ? id : 'newsprint'
}

/** Same order as the public page: share link, visitor choice, owner default, newsprint. */
export function resolveVisitorPalette(input: {
  url?: string | null
  visitor?: string | null
  published?: string | null
}): PublicPaletteId {
  if (isPublicPalette(input.url)) return input.url
  if (isPublicPalette(input.visitor)) return input.visitor
  if (isPublicPalette(input.published)) return input.published
  return 'newsprint'
}

export function emptyPublication(): PublicationView {
  return {
    schema_version: 1,
    public_palette: 'newsprint',
    revision: '0',
    status: 'loading',
    export_command: 'wheretoken profile build ./public-profile --public-palette newsprint',
  }
}

export function bundlePalette(view: PublicationView): PublicPaletteId | '' {
  return isPublicPalette(view.bundle_palette) ? view.bundle_palette : ''
}

export function publicationPending(selected: PublicPaletteId, view: PublicationView): boolean {
  if (view.status === 'loading' || view.status === 'hosted') return false
  if (view.status === 'saved_locally' || view.status === 'pending' || view.status === 'failed') return true
  const shipped = bundlePalette(view)
  if (!shipped) return selected !== view.public_palette
  return selected !== shipped
}

export function statusCopy(view: PublicationView): string {
  switch (view.status) {
    case 'loading':
      return '正在读取公开 Profile。'
    case 'hosted':
      return '公开 Profile 由本机 wheretoken profile build 生成。'
    case 'failed':
      return view.error || '写入失败。'
    case 'ready_to_publish':
      return '本地公开包已生成。线上页面要等这些文件被提交并推送后才会变。'
    case 'saved_locally':
      return '已记在本机。还没有写入本地公开包。'
    case 'pending':
      return '本机选择已保存。本地公开包仍是上一份。'
    default:
      return '公开 Profile 默认是 Newsprint。'
  }
}

export function previewIntensity(index: number): number {
  const n = Math.imul(index + 1, 1103515245) + 12345
  const u = ((n >>> 0) % 1000) / 1000
  if (u < 0.46) return 0
  return (u - 0.46) / 0.54
}

export function previewIntensities(): number[] {
  return Array.from({ length: 371 }, (_, i) => previewIntensity(i))
}

export function heatHex(palette: PublicPaletteId, intensity: number): string {
  const stops = STOPS[palette]
  const t = Math.min(Math.max(intensity, 0), 1)
  let from = stops[0]
  let to = stops[stops.length - 1]
  for (let i = 1; i < stops.length; i++) {
    if (t <= stops[i][0]) {
      from = stops[i - 1]
      to = stops[i]
      break
    }
  }
  const span = to[0] - from[0] || 1
  const u = (t - from[0]) / span
  const color = from[1].map((value, i) => value + (to[1][i] - value) * u)
  return oklchHex(color[0], color[1], color[2])
}

function oklchHex(l: number, c: number, h: number): string {
  const radians = (h * Math.PI) / 180
  const a = c * Math.cos(radians)
  const b = c * Math.sin(radians)
  const lRoot = l + 0.3963377774 * a + 0.2158037573 * b
  const mRoot = l - 0.1055613458 * a - 0.0638541728 * b
  const sRoot = l - 0.0894841775 * a - 1.291485548 * b
  const ll = lRoot ** 3
  const mm = mRoot ** 3
  const ss = sRoot ** 3
  const rgb = [
    4.0767416621 * ll - 3.3077115913 * mm + 0.2309699292 * ss,
    -1.2684380046 * ll + 2.6097574011 * mm - 0.3413193965 * ss,
    -0.0041960863 * ll - 0.7034186147 * mm + 1.707614701 * ss,
  ]
  const channel = (linear: number) => {
    const encoded = linear <= 0.0031308 ? 12.92 * linear : 1.055 * linear ** (1 / 2.4) - 0.055
    return Math.round(Math.min(Math.max(encoded, 0), 1) * 255)
      .toString(16)
      .padStart(2, '0')
  }
  return '#' + rgb.map(channel).join('')
}

export async function loadPublication(hosted: boolean): Promise<PublicationView> {
  if (hosted) {
    return {
      ...emptyPublication(),
      status: 'hosted',
    }
  }
  const res = await fetch('/api/public-profile', { cache: 'no-store' })
  if (!res.ok) throw new Error('could not read public profile appearance')
  return normalizePublication(await res.json())
}

export async function applyPublication(palette: PublicPaletteId): Promise<PublicationView> {
  const res = await fetch('/api/public-profile', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ public_palette: palette }),
  })
  if (!res.ok) throw new Error('could not apply public profile appearance')
  return normalizePublication(await res.json())
}

function normalizePublication(raw: unknown): PublicationView {
  const body = raw && typeof raw === 'object' ? (raw as Partial<PublicationView>) : {}
  const status = body.status
  const known: PublicationStatus[] = [
    'unconfigured',
    'saved_locally',
    'pending',
    'ready_to_publish',
    'failed',
    'hosted',
    'loading',
  ]
  return {
    schema_version: 1,
    public_palette: coercePalette(body.public_palette),
    revision: typeof body.revision === 'string' ? body.revision : '0',
    updated_at: body.updated_at,
    bundle_palette: isPublicPalette(body.bundle_palette) ? body.bundle_palette : undefined,
    bundle_revision: body.bundle_revision,
    status: known.includes(status as PublicationStatus) ? (status as PublicationStatus) : 'failed',
    preview_path: typeof body.preview_path === 'string' ? body.preview_path : undefined,
    export_command:
      typeof body.export_command === 'string' && body.export_command
        ? body.export_command
        : emptyPublication().export_command,
    error: typeof body.error === 'string' ? body.error : undefined,
  }
}
