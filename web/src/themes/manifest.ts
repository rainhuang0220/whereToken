export const STORAGE_KEY = 'wheretoken.theme'
export const DEFAULT_THEME = 'kiln' as const

export const THEME_IDS = ['kiln', 'moss', 'porcelain', 'jiang', 'day', 'ink', 'cartoon', 'ledger'] as const
export type ThemeId = (typeof THEME_IDS)[number]

export const REQUIRED_TOKENS = [
  'void',
  'clay',
  'mortar',
  'ember-1',
  'ember-2',
  'ember-3',
  'ember-4',
  'bone',
  'ash',
  'copper',
  'warn',
  'glow',
  'hi',
  'lo',
  'scheme',
] as const

export type TokenName = (typeof REQUIRED_TOKENS)[number]
export type ThemeTokens = Record<TokenName, string>

export const CHROME_TOKENS = [
  'brick-radius',
  'key-radius',
  'lever-radius',
  'wall-radius',
  'cell',
  'gap',
  'font-display',
  'font-ui',
  'font-mono',
] as const

export type ChromeName = (typeof CHROME_TOKENS)[number]
export type ThemeChrome = Record<ChromeName, string>

export type ThemePack = {
  id: ThemeId
  mark: string
  name: string
  blurb: readonly string[]
  tokens: ThemeTokens
  chrome: ThemeChrome
}

const FORGE_CHROME: ThemeChrome = {
  'brick-radius': '2px',
  'key-radius': '4px',
  'lever-radius': '0px',
  'wall-radius': '0px',
  cell: '13px',
  gap: '4px',
  'font-display': "'Big Shoulders Display', 'Chiron Hei HK', sans-serif",
  'font-ui': "'Chiron Hei HK', 'Source Han Sans SC', 'Noto Sans SC', sans-serif",
  'font-mono': "'Martian Mono', ui-monospace, monospace",
}

export const themes: ThemePack[] = [
  {
    id: 'kiln',
    mark: '窑',
    name: '窑',
    blurb: ['token 烧得快，像窑。', '焦黄到炭黑：速度、热、动。'],
    chrome: FORGE_CHROME,
    tokens: {
      void: '#070504',
      clay: '#3a2c22',
      mortar: '#120c09',
      'ember-1': '#4a2714',
      'ember-2': '#9a3a0d',
      'ember-3': '#e85a10',
      'ember-4': '#ffc44a',
      bone: '#e8dcc8',
      ash: '#8a7a68',
      copper: '#c47a3a',
      warn: '#ff6b3d',
      glow: '#e85a10',
      hi: '#ffdca0',
      lo: '#000000',
      scheme: 'dark',
    },
  },
  {
    id: 'moss',
    mark: '苔',
    name: '苔',
    blurb: ['清新，又复古一点的现代。', '青苔贴在石头上，绿不必吵。'],
    chrome: FORGE_CHROME,
    tokens: {
      void: '#eef3e4',
      clay: '#b7c6a4',
      mortar: '#d5deca',
      'ember-1': '#8fb87a',
      'ember-2': '#5a9450',
      'ember-3': '#34743a',
      'ember-4': '#1a4a24',
      bone: '#172016',
      ash: '#3d4f3c',
      copper: '#4e6a3c',
      warn: '#a33b18',
      glow: '#7aaa5a',
      hi: '#f7fbf2',
      lo: '#142018',
      scheme: 'light',
    },
  },
  {
    id: 'porcelain',
    mark: '瓷',
    name: '瓷',
    blurb: ['青花瓷。', '白地、钴料，砖从素坯走到近墨。'],
    chrome: FORGE_CHROME,
    tokens: {
      void: '#f6f2eb',
      clay: '#d0c4b4',
      mortar: '#e6ddd0',
      'ember-1': '#b7c9c8',
      'ember-2': '#7a9ea8',
      'ember-3': '#3d6580',
      'ember-4': '#1a2430',
      bone: '#1c1b19',
      ash: '#5a5650',
      copper: '#3a6570',
      warn: '#8c2e2e',
      glow: '#8aadb8',
      hi: '#ffffff',
      lo: '#1a1814',
      scheme: 'light',
    },
  },
  {
    id: 'jiang',
    mark: '绛',
    name: '绛',
    blurb: ['粉和黑。', '夜里的现代，口红那种亮，不是糖果。'],
    chrome: FORGE_CHROME,
    tokens: {
      void: '#140910',
      clay: '#3e2430',
      mortar: '#1c0c14',
      'ember-1': '#5c2040',
      'ember-2': '#a01c58',
      'ember-3': '#e02878',
      'ember-4': '#ff8ab0',
      bone: '#f4e6ec',
      ash: '#c49aac',
      copper: '#d46286',
      warn: '#ff8a64',
      glow: '#e02878',
      hi: '#ffd0e0',
      lo: '#000000',
      scheme: 'dark',
    },
  },
  {
    id: 'day',
    mark: '昼',
    name: '昼',
    blurb: ['蓝白黑。产品界面的现代。', '不是霓虹。'],
    chrome: FORGE_CHROME,
    tokens: {
      void: '#ffffff',
      clay: '#d4dce8',
      mortar: '#e8edf4',
      'ember-1': '#c5daf8',
      'ember-2': '#6ea8f0',
      'ember-3': '#2f6fed',
      'ember-4': '#1d4ed8',
      bone: '#0c0d10',
      ash: '#4a5160',
      copper: '#3d5f96',
      warn: '#b42318',
      glow: '#2f6fed',
      hi: '#ffffff',
      lo: '#07080c',
      scheme: 'light',
    },
  },
  {
    id: 'ink',
    mark: '墨',
    name: '墨',
    blurb: ['黑白。印成报纸也长这样。', '作者最喜欢的配色。', '没别的好说。'],
    chrome: FORGE_CHROME,
    tokens: {
      void: '#ffffff',
      clay: '#d4d4d4',
      mortar: '#ececec',
      'ember-1': '#b5b5b5',
      'ember-2': '#8a8a8a',
      'ember-3': '#4a4a4a',
      'ember-4': '#111111',
      bone: '#111111',
      ash: '#595959',
      copper: '#5c5c5c',
      warn: '#1a1a1a',
      glow: '#888888',
      hi: '#ffffff',
      lo: '#000000',
      scheme: 'light',
    },
  },
  {
    id: 'cartoon',
    mark: '漫',
    name: '漫',
    blurb: ['圆砖、粗字。', '观察台也可以玩一下。', '合计还是要能读。'],
    chrome: {
      'brick-radius': '7px',
      'key-radius': '10px',
      'lever-radius': '999px',
      'wall-radius': '14px',
      cell: '14px',
      gap: '5px',
      'font-display': "'Bagel Fat One', 'M PLUS Rounded 1c', 'Chiron Hei HK', sans-serif",
      'font-ui': "'M PLUS Rounded 1c', 'Chiron Hei HK', sans-serif",
      'font-mono': "'Martian Mono', ui-monospace, monospace",
    },
    tokens: {
      void: '#fff1d6',
      clay: '#e0c08a',
      mortar: '#f3ddb4',
      'ember-1': '#f0b24a',
      'ember-2': '#e06a1c',
      'ember-3': '#c43a10',
      'ember-4': '#7a1408',
      bone: '#1a100c',
      ash: '#5a4034',
      copper: '#c44c18',
      warn: '#b41810',
      glow: '#e06a1c',
      hi: '#fffaf0',
      lo: '#120806',
      scheme: 'light',
    },
  },
  {
    id: 'ledger',
    mark: '端',
    name: '端',
    blurb: ['终端账本。', '等宽、直角、磷光。', '墙仍是平的。'],
    chrome: {
      'brick-radius': '0px',
      'key-radius': '2px',
      'lever-radius': '0px',
      'wall-radius': '0px',
      cell: '11px',
      gap: '3px',
      'font-display': "'Share Tech Mono', 'IBM Plex Mono', ui-monospace, monospace",
      'font-ui': "'IBM Plex Mono', 'Martian Mono', ui-monospace, monospace",
      'font-mono': "'IBM Plex Mono', ui-monospace, monospace",
    },
    tokens: {
      void: '#0b0d09',
      clay: '#1c2414',
      mortar: '#070806',
      'ember-1': '#2e3a16',
      'ember-2': '#5c6a1c',
      'ember-3': '#a8b028',
      'ember-4': '#e8ec70',
      bone: '#dce8c0',
      ash: '#8c9870',
      copper: '#a8b048',
      warn: '#e89838',
      glow: '#c8d050',
      hi: '#f4f8c8',
      lo: '#000000',
      scheme: 'dark',
    },
  },
]

export function isThemeId(value: unknown): value is ThemeId {
  return typeof value === 'string' && (THEME_IDS as readonly string[]).includes(value)
}

export function resolveThemeId(raw: string | null | undefined): ThemeId {
  return isThemeId(raw) ? raw : DEFAULT_THEME
}

export function themeStylesheet(): string {
  return themes
    .map((theme) => {
      const sel =
        theme.id === DEFAULT_THEME ? `:root,[data-theme="${theme.id}"]` : `[data-theme="${theme.id}"]`
      const colors = REQUIRED_TOKENS.map((key) => {
        if (key === 'scheme') return `color-scheme:${theme.tokens.scheme}`
        return `--${key}:${theme.tokens[key]}`
      })
      const chrome = CHROME_TOKENS.map((key) => `--${key}:${theme.chrome[key]}`)
      return `${sel}{${[...colors, ...chrome].join(';')}}`
    })
    .join('')
}
