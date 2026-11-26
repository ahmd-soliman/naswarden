import type { UpdateInfo } from './usePoolSocket'

/** Short badge text for the Server card, or null when there is nothing to show. */
export function updateBadge(u: UpdateInfo | undefined): string | null {
  if (!u?.available) return null
  return u.version ? `Update ${u.version}` : 'Update available'
}

/** "2 apps have updates", or null when none. */
export function appsUpdateText(u: UpdateInfo | undefined): string | null {
  const n = u?.apps_with_updates ?? 0
  if (n <= 0) return null
  return n === 1 ? '1 app has an update' : `${n} apps have updates`
}

/** Only http(s) links go into an href. */
export function safeLink(url: string | undefined): string | null {
  return url && /^https?:\/\//i.test(url) ? url : null
}
