// Demo mode is a build-time switch (VITE_DEMO=1) for the public static
// site: data comes from the committed sample payloads, never from a
// backend, and scan/community actions are inert.
export function isDemo(): boolean {
  return import.meta.env.VITE_DEMO === '1'
}
