export type DashboardMode = 'demo' | 'hosted' | 'local'

export function dashboardMode(): DashboardMode {
  if (import.meta.env.VITE_DEMO === '1') return 'demo'
  if (import.meta.env.VITE_HOSTED === '1') return 'hosted'
  return 'local'
}

export function isDemo(): boolean {
  return dashboardMode() === 'demo'
}

export function isHosted(): boolean {
  return dashboardMode() === 'hosted'
}
