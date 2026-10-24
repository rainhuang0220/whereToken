export type Lang = 'en' | 'zh-CN'

const STORAGE = 'wt-lang'

const messages: Record<Lang, Record<string, string>> = {
  en: {
    'lang.group': 'Language',
    'status.loading': 'Reading the public profile.',
    'status.hosted': 'The public profile is built on this computer with wheretoken profile build.',
    'status.failed': 'The write failed.',
    'status.ready_to_publish': 'The local public bundle is ready. The live page changes after these files are committed and pushed.',
    'status.saved_locally': 'Saved on this computer. The local public bundle has not been written yet.',
    'status.pending': 'The choice on this computer is saved. The local public bundle is still the previous one.',
    'status.unconfigured': 'The public profile defaults to Newsprint.',
    'phase.unsaved': 'Unsaved',
    'phase.saved_locally': 'Saved on this computer',
    'phase.preflight': 'Waiting for preflight',
    'phase.awaiting_approval': 'Waiting for you to confirm',
    'phase.updating_bundle': 'Updating the public bundle',
    'phase.pushing_product': 'Pushing whereToken',
    'phase.waiting_pages': 'Waiting for Pages',
    'phase.live_verified': 'The live page is verified',
    'phase.updating_readme': 'Updating the personal README',
    'phase.verified': 'The GitHub profile is verified',
    'phase.partially_published': 'The public page is live. The personal README was not updated.',
    'phase.failed': 'Publish failed',
    'appearance.h': 'Public profile appearance',
    'appearance.lead': 'Choose Cobalt, Magenta, or Newsprint. Saving on this computer does not publish. Publishing to the GitHub profile asks again.',
    'appearance.bundle': 'Public bundle {name}',
    'appearance.bundleMissing': 'Not built',
    'appearance.editing': 'Editing {name}',
    'appearance.unsaved': 'Unsaved',
    'appearance.group': 'Public profile appearance',
    'appearance.frame': 'Preview size',
    'appearance.desktop': 'Desktop',
    'appearance.mobile': 'Phone',
    'appearance.previewLabel': 'Appearance preview',
    'appearance.save': 'Save on this computer',
    'appearance.saving': 'Writing',
    'appearance.publish': 'Publish to my GitHub profile',
    'appearance.preview': 'Preview public profile',
    'appearance.selected': 'Selected {palette}. Live is {live}.',
    'appearance.liveUnknown': 'not read yet',
    'appearance.check': 'Check {validation} · {provenance}',
    'appearance.cache': 'Cache {key}',
    'appearance.staticSvg': 'The image on the GitHub profile is a static SVG. The live paper is only on the open public page.',
    'appearance.confirm': 'Confirm publish to both repositories',
    'appearance.retry': 'Retry the GitHub profile update',
    'appearance.productCommit': 'whereToken commit',
    'appearance.pages': 'Pages',
    'appearance.readmeCommit': 'Personal README commit',
    'appearance.livePage': 'Public page',
    'appearance.hosted': "This demo site cannot publish. Open the link below on the computer running wheretoken serve. 127.0.0.1 on a phone is not that computer.",
    'appearance.alsoPush': 'Also push {item}',
    'appearance.readFailed': 'Could not read the public profile on this computer.',
    'appearance.writeFailed': 'The write failed.',
    'appearance.preflightFailed': 'Preflight failed. Check that wheretoken is running on this computer.',
    'appearance.publishRefused': 'The publish request was refused.',
    'appearance.retryRefused': 'The retry was refused. Run preflight again.',
    'appearance.noBundle': 'There is no local public bundle yet. The wall above is an appearance preview.',
  },
  'zh-CN': {
    'lang.group': '语言',
    'status.loading': '正在读取公开 Profile。',
    'status.hosted': '公开 Profile 由本机 wheretoken profile build 生成。',
    'status.failed': '写入失败。',
    'status.ready_to_publish': '本地公开包已生成。线上页面要等这些文件被提交并推送后才会变。',
    'status.saved_locally': '已记在本机。还没有写入本地公开包。',
    'status.pending': '本机选择已保存。本地公开包仍是上一份。',
    'status.unconfigured': '公开 Profile 默认是 Newsprint。',
    'phase.unsaved': '未保存',
    'phase.saved_locally': '本机已保存',
    'phase.preflight': '待预检',
    'phase.awaiting_approval': '待本人确认',
    'phase.updating_bundle': '正在更新公开包',
    'phase.pushing_product': '正在推送 whereToken',
    'phase.waiting_pages': '等待 Pages',
    'phase.live_verified': '线上已核验',
    'phase.updating_readme': '正在更新个人 README',
    'phase.verified': 'GitHub 主页已核验',
    'phase.partially_published': '公开页已上线，个人 README 未更新',
    'phase.failed': '发布失败',
    'appearance.h': '公开 Profile 外观',
    'appearance.lead': '选 Cobalt、Magenta 或 Newsprint。仅保存本机不会上线。发布到 GitHub 主页需要再确认一次。',
    'appearance.bundle': '公开包 {name}',
    'appearance.bundleMissing': '未生成',
    'appearance.editing': '编辑 {name}',
    'appearance.unsaved': '未保存',
    'appearance.group': '公开 Profile 外观',
    'appearance.frame': '预览尺寸',
    'appearance.desktop': '桌面',
    'appearance.mobile': '手机',
    'appearance.previewLabel': '外观预览',
    'appearance.save': '仅保存本机',
    'appearance.saving': '正在写入',
    'appearance.publish': '发布到我的 GitHub 主页',
    'appearance.preview': '预览公开 Profile',
    'appearance.selected': '选择 {palette}。线上现在是 {live}。',
    'appearance.liveUnknown': '还没读到',
    'appearance.check': '校验 {validation} · {provenance}',
    'appearance.cache': '缓存 {key}',
    'appearance.staticSvg': 'GitHub 主页上的图是静态 SVG。实时纸面只在打开的公开页里。',
    'appearance.confirm': '确认发布到这两个仓库',
    'appearance.retry': '重试更新 GitHub 主页',
    'appearance.productCommit': 'whereToken 提交',
    'appearance.pages': 'Pages',
    'appearance.readmeCommit': '个人 README 提交',
    'appearance.livePage': '公开页',
    'appearance.hosted': '这个演示站不能发布。在已经运行 wheretoken serve 的电脑上打开下面的链接。手机上的 127.0.0.1 不是那台电脑。',
    'appearance.alsoPush': '将一并推送 {item}',
    'appearance.readFailed': '读不到本机公开 Profile 配置。',
    'appearance.writeFailed': '写入失败。',
    'appearance.preflightFailed': '预检失败。确认本机 wheretoken 正在运行。',
    'appearance.publishRefused': '发布请求被拒绝。',
    'appearance.retryRefused': '重试被拒绝。请重新预检。',
    'appearance.noBundle': '还没有本地公开包。上面是外观预览。',
  },
}

export function validLang(value: string | null | undefined): Lang | '' {
  return value === 'en' || value === 'zh-CN' ? value : ''
}

export function resolveLang(search: string): Lang {
  const query = search.startsWith('?') ? search.slice(1) : search
  const fromURL = validLang(new URLSearchParams(query).get('lang'))
  if (fromURL) return fromURL
  try {
    const saved = validLang(globalThis.localStorage?.getItem(STORAGE))
    if (saved) return saved
  } catch {
    // Storage can be unavailable. The page still starts in English.
  }
  return 'en'
}

export function persistLang(lang: Lang): void {
  try {
    globalThis.localStorage?.setItem(STORAGE, lang)
  } catch {
    // An explicit choice that cannot be stored still applies for this view.
  }
}

export function t(lang: Lang, key: string, vars?: Record<string, string>): string {
  const table = messages[lang] || messages.en
  let text = table[key] ?? messages.en[key] ?? key
  if (vars) {
    for (const [name, value] of Object.entries(vars)) {
      text = text.split('{' + name + '}').join(value)
    }
  }
  return text
}
