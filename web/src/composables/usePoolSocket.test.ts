import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// The composable registers its socket in onMounted; run it immediately so it
// can be exercised without mounting a component.
vi.mock('vue', async () => ({
  ...(await vi.importActual<typeof import('vue')>('vue')),
  onMounted: (fn: () => void) => fn(),
  onBeforeUnmount: () => {},
}))

import { usePoolSocket } from './usePoolSocket'

class FakeSocket {
  static instances: FakeSocket[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  closed = false
  constructor(public url: string) {
    FakeSocket.instances.push(this)
  }
  close() {
    this.closed = true
    this.onclose?.()
  }
  send(msg: unknown) {
    this.onmessage?.({ data: JSON.stringify(msg) })
  }
}

const last = () => FakeSocket.instances[FakeSocket.instances.length - 1]

beforeEach(() => {
  FakeSocket.instances = []
  vi.useFakeTimers()
  vi.stubGlobal('WebSocket', FakeSocket)
  vi.stubGlobal('window', { location: { protocol: 'http:', host: 'nas:8080' } })
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

const state = (over: Record<string, unknown> = {}) => ({
  type: 'state',
  updated_at: 1700000000,
  server: null,
  pools: [{ name: 'tank' }],
  datasets: [],
  containers: null,
  vms: null,
  ...over,
})

describe('usePoolSocket', () => {
  it('connects to /ws on the page host, with wss on https', () => {
    usePoolSocket()
    expect(last().url).toBe('ws://nas:8080/ws')
    vi.stubGlobal('window', { location: { protocol: 'https:', host: 'nas' } })
    usePoolSocket()
    expect(last().url).toBe('wss://nas/ws')
  })

  it('applies a state message and defaults the optional lists to empty', () => {
    const s = usePoolSocket()
    last().onopen?.()
    expect(s.connected.value).toBe(true)
    last().send(state({ stale_sources: ['docker'] }))
    expect(s.pools.value).toHaveLength(1)
    expect(s.containers.value).toEqual([]) // null from the backend must not reach the UI
    expect(s.disks.value).toEqual([])
    expect(s.alerts.value).toEqual([])
    expect(s.updatedAt.value).toBe(1700000000)
    expect(s.staleSources.value).toEqual(['docker'])
  })

  it('an error message sets the reason and keeps the data; the next state clears it', () => {
    const s = usePoolSocket()
    last().send(state())
    last().send({ type: 'error', message: 'could not refresh pools from TrueNAS: boom' })
    expect(s.serverError.value).toContain('boom')
    expect(s.pools.value).toHaveLength(1) // last good data stays visible
    last().send(state())
    expect(s.serverError.value).toBeNull()
  })

  it('reconnects with a doubling backoff, capped at 30s, and resets on open', () => {
    usePoolSocket()
    const delays: number[] = []
    for (let i = 0; i < 7; i++) {
      const before = FakeSocket.instances.length
      last().onclose?.()
      // the retry has not happened yet
      expect(FakeSocket.instances.length).toBe(before)
      let waited = 0
      while (FakeSocket.instances.length === before) {
        vi.advanceTimersByTime(500)
        waited += 500
      }
      delays.push(waited)
    }
    expect(delays).toEqual([1000, 2000, 4000, 8000, 16000, 30000, 30000])

    last().onopen?.() // a successful connection resets the backoff
    const before = FakeSocket.instances.length
    last().onclose?.()
    vi.advanceTimersByTime(1000)
    expect(FakeSocket.instances.length).toBe(before + 1)
  })

  it('an error event closes the socket so the retry path runs', () => {
    usePoolSocket()
    const sock = last()
    sock.onerror?.()
    expect(sock.closed).toBe(true)
  })
})
