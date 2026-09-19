import { ref, watch } from 'vue'

export type ViewMode = 'cards' | 'table'

// Remembers a per-list card/table choice in localStorage. Storage can be
// unavailable or throw (private windows, blocked site data), so every access
// is guarded and the page works without it.
export function useViewMode(key: string, fallback: ViewMode) {
  const storageKey = `naswarden:view:${key}`
  let initial = fallback
  try {
    const saved = localStorage.getItem(storageKey)
    if (saved === 'cards' || saved === 'table') initial = saved
  } catch {
    /* no storage: use the default */
  }
  const mode = ref<ViewMode>(initial)
  watch(mode, (v) => {
    try {
      localStorage.setItem(storageKey, v)
    } catch {
      /* ignore */
    }
  })
  return mode
}
