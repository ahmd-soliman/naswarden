import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { useViewMode } from './useViewMode'

afterEach(() => vi.unstubAllGlobals())

const fakeStorage = (initial: Record<string, string> = {}) => {
  const data = { ...initial }
  return {
    data,
    getItem: (k: string) => data[k] ?? null,
    setItem: (k: string, v: string) => void (data[k] = v),
  }
}

describe('useViewMode', () => {
  it('uses the fallback when nothing is stored', () => {
    vi.stubGlobal('localStorage', fakeStorage())
    expect(useViewMode('a', 'table').value).toBe('table')
  })
  it('restores a saved choice and ignores junk', () => {
    vi.stubGlobal('localStorage', fakeStorage({ 'naswarden:view:a': 'cards', 'naswarden:view:b': 'grid' }))
    expect(useViewMode('a', 'table').value).toBe('cards')
    expect(useViewMode('b', 'table').value).toBe('table')
  })
  it('saves changes', async () => {
    const st = fakeStorage()
    vi.stubGlobal('localStorage', st)
    const mode = useViewMode('a', 'table')
    mode.value = 'cards'
    await nextTick()
    expect(st.data['naswarden:view:a']).toBe('cards')
  })
  it('still works when storage throws (private window, blocked site data)', async () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('blocked')
      },
    })
    const mode = useViewMode('a', 'cards')
    expect(mode.value).toBe('cards')
    mode.value = 'table'
    await nextTick()
    expect(mode.value).toBe('table')
  })
})
