import type { SummaryPayload } from '../types'

export type HostedPhase =
  | 'logged_in_no_device'
  | 'paired_no_sync'
  | 'synced'
  | 'stale'

export function hostedPhase(payload: SummaryPayload | null | undefined): HostedPhase | '' {
  const hosted = payload?.hosted
  if (!hosted) return ''
  const devices = (hosted.devices ?? []).filter((d) => !d.revoked)
  if (hosted.never_synced) {
    return devices.length === 0 ? 'logged_in_no_device' : 'paired_no_sync'
  }
  const last = hosted.last_sync_at
  if (last && !last.startsWith('0001')) {
    const t = Date.parse(last)
    if (Number.isFinite(t) && Date.now() - t > 36 * 3600 * 1000) return 'stale'
  }
  return 'synced'
}
