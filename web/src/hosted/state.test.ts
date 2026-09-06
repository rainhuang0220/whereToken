import { describe, expect, it } from 'vitest'
import { hostedPhase } from './state'
import type { SummaryPayload } from '../types'

function payload(hosted: SummaryPayload['hosted']): SummaryPayload {
  return { hosted } as SummaryPayload
}

describe('hostedPhase', () => {
  it('logged_in_no_device', () => {
    expect(hostedPhase(payload({ never_synced: true, devices: [] }))).toBe('logged_in_no_device')
  })
  it('paired_no_sync', () => {
    expect(
      hostedPhase(payload({ never_synced: true, devices: [{ id: 'd', label: 'Mac', os: 'darwin', arch: 'arm64' }] })),
    ).toBe('paired_no_sync')
  })
  it('synced', () => {
    expect(
      hostedPhase(
        payload({
          never_synced: false,
          last_sync_at: new Date().toISOString(),
          devices: [{ id: 'd', label: 'Mac', os: 'darwin', arch: 'arm64' }],
        }),
      ),
    ).toBe('synced')
  })
  it('stale after 36h', () => {
    const old = new Date(Date.now() - 40 * 3600 * 1000).toISOString()
    expect(hostedPhase(payload({ never_synced: false, last_sync_at: old }))).toBe('stale')
  })
})
