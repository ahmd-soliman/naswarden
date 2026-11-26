import { ref, watch } from 'vue'

export type Theme = 'system' | 'light' | 'dark'
export type Resolved = 'light' | 'dark'

export const THEME_KEY = 'naswarden:theme'

export function isTheme(v: unknown): v is Theme {
  return v === 'system' || v === 'light' || v === 'dark'
}

/** The theme actually shown: "system" follows the OS. */
export function resolveTheme(theme: Theme, systemPrefersLight: boolean): Resolved {
  if (theme === 'system') return systemPrefersLight ? 'light' : 'dark'
  return theme
}

const lightQuery = () =>
  typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia('(prefers-color-scheme: light)')
    : null

function apply(theme: Theme) {
  const resolved = resolveTheme(theme, lightQuery()?.matches ?? false)
  document.documentElement.dataset.theme = resolved
}

// Remembers the choice in localStorage and sets data-theme on <html>, which
// style.css reads. index.html sets it once before the first paint (so a light
// user never sees a dark flash); this keeps it in step afterwards, including
// a live OS change while "system" is chosen. Storage can be unavailable or
// throw, so every access is guarded and the page works without it.
export function useTheme() {
  let initial: Theme = 'system'
  try {
    const saved = localStorage.getItem(THEME_KEY)
    if (isTheme(saved)) initial = saved
  } catch {
    /* no storage: follow the system */
  }
  const theme = ref<Theme>(initial)
  apply(theme.value)

  watch(theme, (v) => {
    apply(v)
    try {
      localStorage.setItem(THEME_KEY, v)
    } catch {
      /* ignore */
    }
  })

  lightQuery()?.addEventListener('change', () => {
    if (theme.value === 'system') apply('system')
  })
  return theme
}
