export function csrfHeaders(): Record<string, string> {
  const m = document.cookie.match(/(?:^|; )wt_csrf=([^;]*)/)
  const token = m ? decodeURIComponent(m[1]) : ''
  return token ? { 'X-CSRF-Token': token } : {}
}
