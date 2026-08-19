import { costCaption as formatCostCaption, rankCaption as formatRankCaption } from './format'
import type { CommunityView, RankStanding, SliceView } from './types'

export function costCaption(all: Pick<SliceView, 'cost_usd' | 'cost_status' | 'total'>): string {
  return formatCostCaption(all) || '—'
}

export function rankCaption(st?: RankStanding | null): string {
  return formatRankCaption(st ?? undefined)
}

export function rankHint(view?: CommunityView | null, st?: RankStanding | null): string {
  const status = st?.status || ''
  switch (status) {
    case 'ok':
      return view?.note || '社区排名基于参与用户匿名上报的聚合用量，不是经过审计的竞技排行榜。'
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
    default:
      return ''
  }
}
