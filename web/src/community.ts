import { MIN_RANK_PARTICIPANTS } from './format'
import type { CommunityView, RankStanding } from './types'

const RANK_HINT_ZH =
  '社区排名基于参与用户匿名上报的聚合用量，不是全球、全世界或全体 AI 用户排名，也不是经过审计的竞技排行榜。'

export function rankHint(_view?: CommunityView | null, st?: RankStanding | null): string {
  const status = st?.status || ''
  const n = st?.participants
  if (status === 'ok' && n != null && n > 0 && n < MIN_RANK_PARTICIPANTS) {
    return '社区排名暂不可用 · 参与者还不够'
  }
  switch (status) {
    case 'ok':
      return RANK_HINT_ZH
    case 'insufficient_participants':
      return '社区排名暂不可用 · 参与者还不够'
    case 'opted_out':
    case 'disabled':
      return '社区排名已关闭'
    case 'offline':
      return '社区排名未上传 · offline'
    case 'network_error':
      return '社区排名暂不可用 · 服务连不上'
    case 'service_unconfigured':
      return '社区排名暂不可用 · 未配置远程服务'
    case 'no_usage':
    case 'not_ranked':
      return '尚未进入社区排名'
    case 'unavailable':
    default:
      return '社区排名暂不可用'
  }
}
