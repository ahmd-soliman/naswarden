import { onBeforeUnmount, onMounted, ref } from 'vue'

export interface Scan {
  function: string
  state: string
  errors: number
}

export interface Pool {
  name: string
  status: string
  healthy: boolean
  warning: boolean
  size: number
  allocated: number
  free: number
  scan: Scan | null
}

export interface Dataset {
  name: string
  used: number
  quota: number
  quota_source: 'quota' | 'refquota'
}

interface StateMessage {
  type: 'state'
  pools: Pool[]
  datasets: Dataset[]
}

// Connects to naswarden's /ws endpoint and keeps `pools`/`datasets`
// reactive and up to date. Reconnects with backoff on disconnect -- the
// backend pushes on its own schedule and replays the last known state to
// new/reconnecting clients, so a dropped connection is recoverable
// without losing state for long.
export function usePoolSocket() {
  const pools = ref<Pool[]>([])
  const datasets = ref<Dataset[]>([])
  const connected = ref(false)

  let socket: WebSocket | null = null
  let retryDelayMs = 1000

  function connect() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new WebSocket(`${protocol}//${window.location.host}/ws`)

    socket.onopen = () => {
      connected.value = true
      retryDelayMs = 1000
    }

    socket.onmessage = (event) => {
      const msg = JSON.parse(event.data) as StateMessage
      if (msg.type === 'state') {
        pools.value = msg.pools
        datasets.value = msg.datasets
      }
    }

    socket.onclose = () => {
      connected.value = false
      setTimeout(connect, retryDelayMs)
      retryDelayMs = Math.min(retryDelayMs * 2, 30_000)
    }

    socket.onerror = () => {
      socket?.close()
    }
  }

  onMounted(connect)
  onBeforeUnmount(() => socket?.close())

  return { pools, datasets, connected }
}
