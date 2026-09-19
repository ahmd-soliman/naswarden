import { onBeforeUnmount, onMounted, ref } from 'vue'

export interface Scan {
  function: string
  state: string
  errors: number
}

export interface VdevChild {
  disk: string
  status: string
  read_errors: number
  write_errors: number
  checksum_errors: number
  temperature_c?: number | null
  standby?: boolean
}

export interface Vdev {
  name: string
  type: string
  status: string
  children: VdevChild[]
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
  fragmentation: string
  vdevs: Vdev[]
}

export interface Dataset {
  name: string
  used: number
  quota: number
  quota_source: 'quota' | 'refquota'
  mountpoint: string
  compression: string
  compress_ratio: string
  recordsize: string
  encrypted: boolean
  used_by_snapshots: number
}

export interface Mount {
  type: string // "bind" | "volume" | "tmpfs"
  source: string
  destination: string
  read_only: boolean
}

export interface NetworkIP {
  name: string
  ip: string
}

export interface Container {
  name: string
  image: string
  state: string // "running", "exited", ...
  status: string // human-readable, e.g. "Up 12 minutes (healthy)"
  cpu_percent: number
  mem_used: number
  mem_limit: number
  started_at: string
  restart_policy: string
  command: string
  mounts: Mount[]
  networks: NetworkIP[]
  ports: string[]
  stack: string // compose project, '' for a loose container
  compose_file: string
  icon: string // optional `naswarden.icon` label override
  exit_code: number // last exit code, meaningful once stopped
  oom_killed?: boolean // true if terminated by kernel OOM killer
  optional?: boolean // true if labeled naswarden.ignore or naswarden.optional
}

export interface Interface {
  name: string
  link_state: string
  speed: string
  addresses: string[]
}

export interface ServerInfo {
  hostname: string
  version: string
  uptime_seconds: number
  cpu_percent: number
  mem_used: number
  mem_total: number
  load_percent_1: number
  load_percent_5: number
  load_percent_15: number
  arc_bytes: number
  interfaces: Interface[]
  cpu_model: string
  cores: number
  physical_cores: number
  cpu_temp_c: number
}

export interface VM {
  name: string
  type: string // "virtual-machine" | "container"
  manager: 'incus' | 'truenas'
  status: string // "Running" | "Stopped" | ...
  status_code: number
  is_vm: boolean
  os: string
  kernel?: string
  arch?: string
  cpu_cores: number
  cpu_percent: number
  mem_used: number
  mem_total: number
  disk_used: number
  disk_total: number
  disk_pool: string
  ipv4: string[]
  mac?: string
  bridge?: string
  started_at?: string
  auto_start: boolean
  config?: Record<string, string>
  display_port?: number
  web_port?: number
  passthrough?: string[]
}

export interface Alert {
  id: string
  uuid?: string
  level: string // "CRITICAL" | "ERROR" | "WARNING" | "INFO"
  source: string
  klass: string
  formatted: string
  text?: string
  datetime: number // Unix seconds
  dismissed: boolean
}

export interface ReplicationTask {
  id: number
  name: string
  direction: string
  transport: string
  source_datasets: string[]
  target_dataset: string
  target_pool: string
  source_pools: string[]
  state: string
  job_state?: string
  progress_percent?: number
  progress_description?: string
  last_snapshot?: string
  time_started?: number
  time_finished?: number
  duration_seconds?: number
  enabled: boolean
}

interface StateMessage {
  type: 'state'
  updated_at?: number // unix seconds the backend took this snapshot
  stale_sources?: string[] // sources whose data is the previous snapshot (refresh failed)
  server: ServerInfo | null
  pools: Pool[]
  datasets: Dataset[]
  containers: Container[] | null
  vms: VM[] | null
  alerts?: Alert[] | null
  replications?: ReplicationTask[] | null
}

// Connects to naswarden's /ws endpoint and keeps `pools`/`datasets`
// reactive and up to date. Reconnects with backoff on disconnect -- the
// backend pushes on its own schedule and replays the last known state to
// new/reconnecting clients, so a dropped connection is recoverable
// without losing state for long.
export function usePoolSocket() {
  const server = ref<ServerInfo | null>(null)
  const pools = ref<Pool[]>([])
  const datasets = ref<Dataset[]>([])
  const containers = ref<Container[]>([])
  const vms = ref<VM[]>([])
  const alerts = ref<Alert[]>([])
  const replications = ref<ReplicationTask[]>([])
  const connected = ref(false)
  const updatedAt = ref<number | null>(null)
  const staleSources = ref<string[]>([])

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
        server.value = msg.server
        pools.value = msg.pools
        datasets.value = msg.datasets
        containers.value = msg.containers ?? []
        vms.value = msg.vms ?? []
        alerts.value = msg.alerts ?? []
        replications.value = msg.replications ?? []
        updatedAt.value = msg.updated_at ?? null
        staleSources.value = msg.stale_sources ?? []
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

  return { server, pools, datasets, containers, vms, alerts, replications, connected, updatedAt, staleSources }
}
