export const STORAGE_KEY = 'wheretoken.theme'
export const DEFAULT_THEME = 'kiln' as const

export const THEME_IDS = ['kiln', 'moss', 'porcelain', 'jiang', 'qingmo', 'frost'] as const
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

export type ThemePack = {
  id: ThemeId
  mark: string
  name: string
  tokens: ThemeTokens
}

export const themes: ThemePack[] = [
  {
    id: 'kiln',
    mark: '窑',
    name: '窑',
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
    id: 'qingmo',
    mark: '青',
    name: '青墨',
    tokens: {
      void: '#0d1412',
      clay: '#263832',
      mortar: '#08100e',
      'ember-1': '#1a4a42',
      'ember-2': '#227060',
      'ember-3': '#2f9a82',
      'ember-4': '#9ee0c8',
      bone: '#dceae4',
      ash: '#8aa098',
      copper: '#5a8a7c',
      warn: '#e09068',
      glow: '#2f9a82',
      hi: '#c8f0e4',
      lo: '#020806',
      scheme: 'dark',
    },
  },
  {
    id: 'frost',
    mark: '霜',
    name: '霜碳',
    tokens: {
      void: '#121418',
      clay: '#2e333a',
      mortar: '#0c0e12',
      'ember-1': '#3e4752',
      'ember-2': '#5c6e82',
      'ember-3': '#7a96b0',
      'ember-4': '#d4e6f6',
      bone: '#e6e8ec',
      ash: '#9098a2',
      copper: '#6e7884',
      warn: '#d08058',
      glow: '#7a96b0',
      hi: '#f0f4f8',
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
      const body = REQUIRED_TOKENS.map((key) => {
        if (key === 'scheme') return `color-scheme:${theme.tokens.scheme}`
        return `--${key}:${theme.tokens[key]}`
      }).join(';')
      return `${sel}{${body}}`
    })
    .join('')
}
