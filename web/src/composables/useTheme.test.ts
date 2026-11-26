import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { resolveTheme, THEME_KEY, useTheme } from './useTheme'

afterEach(() => vi.unstubAllGlobals())

const fakeStorage = (initial: Record<string, string> = {}) => {
  const data = { ...initial }
  return {
    data,
    getItem: (k: string) => data[k] ?? null,
    setItem: (k: string, v: string) => void (data[k] = v),
  }
}

function fakeDom(prefersLight: boolean) {
  const listeners: Array<() => void> = []
  const mq = {
    matches: prefersLight,
    addEventListener: (_: string, fn: () => void) => void listeners.push(fn),
  }
  const root = { dataset: {} as Record<string, string> }
  vi.stubGlobal('window', { matchMedia: () => mq })
  vi.stubGlobal('document', { documentElement: root })
  return { mq, root, fire: () => listeners.forEach((fn) => fn()) }
}

describe('resolveTheme', () => {
  it('follows the system only for "system"', () => {
    expect(resolveTheme('system', true)).toBe('light')
    expect(resolveTheme('system', false)).toBe('dark')
    expect(resolveTheme('light', false)).toBe('light')
    expect(resolveTheme('dark', true)).toBe('dark')
  })
})

describe('useTheme', () => {
  it('defaults to the system theme', () => {
    vi.stubGlobal('localStorage', fakeStorage())
    const dom = fakeDom(true)
    expect(useTheme().value).toBe('system')
    expect(dom.root.dataset.theme).toBe('light')
  })
  it('restores a saved choice and ignores junk', () => {
    vi.stubGlobal('localStorage', fakeStorage({ [THEME_KEY]: 'dark' }))
    const dom = fakeDom(true)
    expect(useTheme().value).toBe('dark')
    expect(dom.root.dataset.theme).toBe('dark')

    vi.stubGlobal('localStorage', fakeStorage({ [THEME_KEY]: 'purple' }))
    expect(useTheme().value).toBe('system')
  })
  it('applies and saves a change', async () => {
    const st = fakeStorage()
    vi.stubGlobal('localStorage', st)
    const dom = fakeDom(false)
    const theme = useTheme()
    expect(dom.root.dataset.theme).toBe('dark')
    theme.value = 'light'
    await nextTick()
    expect(dom.root.dataset.theme).toBe('light')
    expect(st.data[THEME_KEY]).toBe('light')
  })
  it('follows a live OS change only while on "system"', async () => {
    vi.stubGlobal('localStorage', fakeStorage())
    const dom = fakeDom(false)
    const theme = useTheme()
    dom.mq.matches = true
    dom.fire()
    expect(dom.root.dataset.theme).toBe('light')

    theme.value = 'dark'
    await nextTick()
    dom.mq.matches = false
    dom.mq.matches = true
    dom.fire()
    expect(dom.root.dataset.theme).toBe('dark')
  })
  it('works without storage or matchMedia', () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('blocked')
      },
    })
    vi.stubGlobal('window', {})
    const root = { dataset: {} as Record<string, string> }
    vi.stubGlobal('document', { documentElement: root })
    expect(useTheme().value).toBe('system')
    expect(root.dataset.theme).toBe('dark')
  })
})
