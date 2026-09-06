export const loginHeadline = '你的用量，来自你自己的机器。'
export const loginLede =
  '登录后连接你的 whereToken CLI，随时查看本机 AI Coding / Agent 使用情况。'
export const loginCta = '使用 GitHub 登录'
export const loginPrivacy = '只使用 GitHub 识别你的账号'
export const loginNoRepo = '不请求仓库权限'
export const oauthErrorTitle = '登录没有完成'
export const oauthErrorBody =
  'whereToken 没能完成 GitHub 登录。请重新尝试；如果仍然失败，把下方编号发给维护者。'
export const connectTitle = '连接 whereToken'
export const neverSyncedTitle = '尚未同步用量'
export const neverSyncedBody = '连接 whereToken 后，你的聚合用量会显示在这里。'
export const waitingSyncTitle = '设备已连接'
export const waitingSyncBody = '正在等待首次同步…'
export const staleNote = '正在显示最后一次同步的数据'
export const loginOnce = '登录、连接设备并完成首次同步。'
export const pairTitle = '连接一台设备'
export const pairSuccessTitle = '设备已连接'
export const pairSuccessBody = '可以回到终端。whereToken 会自动继续。'
export const pairConfirm = '连接设备'
export const pairCancel = '取消'
export const pairOpenDash = '打开仪表盘'
export const loginStatus = '登录'
export const pairStatus = '配对'
export const syncWhatTitle = '会同步什么'
export const siteLabel = '项目主页'
export const githubLabel = 'GitHub'
export const projectSiteHref = 'https://rainhuang0220.github.io/whereToken/'
export const githubRepoHref = 'https://github.com/rainhuang0220/whereToken'

export const syncedItems = [
  'token counts',
  'models & vendors',
  'requests / turns',
  'dates',
  'source health',
] as const
export const neverSyncedItems = [
  'prompts',
  'conversations',
  'source code',
  'file paths',
  'API keys',
  'raw databases',
] as const
