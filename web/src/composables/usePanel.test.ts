import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { usePanel } from './usePanel'

// A card on the page: where it is, and where it is after the layout changes.
class FakeElement {
  isConnected = true
  top: number
  after: number
  constructor(top: number, after = top) {
    this.top = top
    this.after = after
  }
  closest() {
    return this
  }
  getBoundingClientRect() {
    return { top: this.top }
  }
}

let classes: Set<string>
let scrollBy: ReturnType<typeof vi.fn>
let startViewTransition: ReturnType<typeof vi.fn> | undefined
let finish: () => void
let reducedMotion: boolean
let focused: FakeElement | null
let openCard: FakeElement | null
let cards: FakeElement[]

// Moves every card to its "after" place, as the reflow would.
const reflow = () => cards.forEach((c) => (c.top = c.after))

beforeEach(() => {
  classes = new Set()
  scrollBy = vi.fn()
  reducedMotion = false
  focused = null
  openCard = null
  cards = []
  startViewTransition = vi.fn((update: () => Promise<void>) => {
    void update()
    return { finished: new Promise<void>((resolve) => (finish = resolve)) }
  })
  vi.stubGlobal('Element', FakeElement)
  vi.stubGlobal('matchMedia', () => ({ matches: reducedMotion }))
  vi.stubGlobal('window', { scrollBy })
  vi.stubGlobal('document', {
    get startViewTransition() {
      return startViewTransition
    },
    documentElement: { classList: { add: (c: string) => classes.add(c), remove: (c: string) => classes.delete(c) } },
    get activeElement() {
      return focused
    },
    querySelector: () => openCard,
  })
})

afterEach(() => vi.unstubAllGlobals())

const flush = () => new Promise((resolve) => setTimeout(resolve))

function setup(open: string | null, wide = true) {
  const target = ref<string | null>(open)
  return { target, ...usePanel(target, ref(wide)) }
}

describe('usePanel toggle', () => {
  it('opens a closed card and closes an open one', async () => {
    const p = setup(null)
    p.toggle('a', false)
    await flush()
    expect(p.target.value).toBe('a')
    p.toggle('a', true)
    await flush()
    expect(p.target.value).toBeNull()
  })
})

describe('usePanel select', () => {
  it('switches cards without a transition while the panel stays open', () => {
    const p = setup('a')
    p.select('b')
    expect(p.target.value).toBe('b')
    expect(startViewTransition).not.toHaveBeenCalled()
  })

  it('needs no transition when the panel covers the content', () => {
    const p = setup(null, false)
    p.select('a')
    expect(p.target.value).toBe('a')
    expect(startViewTransition).not.toHaveBeenCalled()
  })

  it('docks through a view transition, marking the page while it runs', async () => {
    const p = setup(null)
    p.select('a')
    expect(startViewTransition).toHaveBeenCalledOnce()
    expect(classes.has('docking')).toBe(true)
    await flush()
    expect(p.target.value).toBe('a')
    finish()
    await flush()
    expect(classes.has('docking')).toBe(false)
  })

  it('docks at once without view transitions or with reduced motion', async () => {
    startViewTransition = undefined
    const p = setup(null)
    p.select('a')
    await flush()
    expect(p.target.value).toBe('a')

    startViewTransition = vi.fn()
    reducedMotion = true
    p.select(null)
    await flush()
    expect(p.target.value).toBeNull()
    expect(startViewTransition).not.toHaveBeenCalled()
    expect(classes.has('docking')).toBe(false)
  })
})

describe('usePanel keeps the clicked card in place', () => {
  // The reflow happens when Vue renders the new target: hook it to the
  // target change.
  function setupReflowing(open: string | null) {
    const p = setup(open)
    startViewTransition = undefined
    const set = p.select
    return {
      ...p,
      select: (t: string | null) => {
        set(t)
        reflow()
      },
    }
  }

  it('scrolls by as much as the pressed card moved', async () => {
    const card = new FakeElement(300, 520)
    cards = [card]
    const p = setupReflowing(null)
    p.onPress({ target: card as unknown as EventTarget })
    p.select('a')
    await flush()
    expect(scrollBy).toHaveBeenCalledWith({ top: 220, behavior: 'instant' })
  })

  it('prefers the pressed card over the focused one', async () => {
    const pressed = new FakeElement(100, 150)
    focused = new FakeElement(400, 900)
    cards = [pressed, focused]
    const p = setupReflowing(null)
    p.onPress({ target: pressed as unknown as EventTarget })
    p.select('a')
    await flush()
    expect(scrollBy).toHaveBeenCalledWith({ top: 50, behavior: 'instant' })
  })

  it('uses the focused card for the keyboard, then the open one', async () => {
    focused = new FakeElement(200, 260)
    cards = [focused]
    const p = setupReflowing(null)
    p.select('a')
    await flush()
    expect(scrollBy).toHaveBeenLastCalledWith({ top: 60, behavior: 'instant' })

    focused = null
    openCard = new FakeElement(500, 380)
    cards = [openCard]
    p.select(null)
    await flush()
    expect(scrollBy).toHaveBeenLastCalledWith({ top: -120, behavior: 'instant' })
  })

  it('forgets a press once used, and ignores a card no longer on the page', async () => {
    const gone = new FakeElement(100, 300)
    gone.isConnected = false
    const p = setupReflowing(null)
    p.onPress({ target: gone as unknown as EventTarget })
    p.select('a')
    await flush()
    expect(scrollBy).not.toHaveBeenCalled()
  })
})
