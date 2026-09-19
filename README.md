# NasWarden

A fast, glanceable, real-time monitoring dashboard and telemetry exporter for **TrueNAS SCALE** homelabs.

**NasWarden** was built to fill a critical gap: standard monitoring tools (`df`, `node-exporter`) only track per-*mountpoint* disk usage and are completely blind to **ZFS dataset-level quota and refquota limits**. On TrueNAS systems where app data, backups, and user home directories are capped using dataset properties, exhaustion happens silently without mountpoint-level alerts.

NasWarden monitors dataset quota utilization in real time, and expands into a unified, single-page operational board covering the entire host:
- **ZFS Dataset Quotas & Storage Pools**
- **Hardware Telemetry & Host Pressure**
- **Docker Compose Stacks & Container Health**
- **Virtual Machines & Containers (TrueNAS Native KVM + Incus KVM/LXC)**
- **Prometheus Metrics & PromQL Linear Exhaustion Forecasting**

---

## Key Features

### 1. ZFS Quota Utilization & Pool Health
- **Dataset Quota Tracking**: Discovers all datasets with `quota` or `refquota` configured. Displays live space consumed vs. limit, quota source, compression algorithm, ratio, record size, and space held by snapshots.
- **Visual Thresholds**: Color-coded utilization bars (green `< 70%`, yellow `70%–89%`, red `≥ 90%`).
- **Pool Topology & Scans**: Tracks pool status (`ONLINE`, `DEGRADED`, `FAULTED`), vdev topology (mirrors, bare disks, stripes), per-disk read/write/checksum errors, fragmentation, and scrub/resilver progress.

### 2. Host Resources & ZFS ARC Breakdown
- **Hardware Telemetry**: System pressure (normalized CPU load), package CPU temperature, and physical vs. logical core counts.
- **ZFS ARC Cache Separation**: Splits ZFS Adaptive Replacement Cache (ARC) out from regular memory usage so reclaimable filesystem cache isn't mistaken for runaway application memory.
- **Active Network Interfaces**: Live link status, connection speeds, and IP addresses.

### 2b. Disks & Network
- **Disks tab**: every physical drive with model, capacity, type, pool, current temperature with its 7-day range, live read/write rate (from ZFS counters, pool members) and ZFS error counts. Click a disk for details. Temperature turns amber at 55 °C and red at 60 °C, matching the shipped alert rules.
- **Network**: total live throughput on the Server card, and per-interface received/sent rates (plus link utilisation for physical interfaces) in the Server panel.
- SMART health is not shown: TrueNAS 25.10 removed SMART from its API.

### 3. Docker Compose Stacks
- **Project Aggregation**: Automatically groups containers by their Docker Compose project (`com.docker.compose.project`), reporting aggregate CPU/memory and member health (e.g. `4/4 running`).
- **Graceful Stops vs. Crashes**: Distinguishes intentional exits (`exit 0`, `SIGTERM 143`, `SIGKILL 137`) from abnormal crashes (error exit codes, kernel `OOMKilled`), keeping clean stops from triggering false positive health alerts.
- **Container Details**: Bind mounts vs. named volumes, network IP addresses, published ports, restart policies, and commands.
- **Customization Labels**:
  - `naswarden.icon`: Override or specify the icon slug (e.g. `jellyfin`, `postgres`, `caddy`).
  - `naswarden.optional`: Mark non-critical or batch services to exclude from stack health degradation counts.

### 4. Unified Virtual Machines & Containers (KVM + LXC)
- **TrueNAS Native VMs**: Connects to TrueNAS middleware to monitor native KVM VMs (e.g. Windows VMs with PCI passthrough). Discovers guest OS, CPU socket/core topology, ZFS zvol backing storage, SPICE display/web ports, and hardware passthrough devices (e.g. NVIDIA GeForce GPUs).
- **Incus (LXD) Integration**: Connects via mTLS to monitor Incus virtual machines and system containers. Reports vCPU counts, memory limits, guest IP addresses, network bridges, and boot autostart.
- **4-Tier Icon Pipeline**: Automatically resolves icons from user overrides (`user.icon` / `naswarden.icon`), workload names (`k8s-*`, `gitlab-*`, `openstack`), guest OS (`windows`, `ubuntu`, `debian`, `arch`, `alpine`), or platform fallbacks.

### 5. Glanceable UI & Accessibility
- **Responsive Layout**: Sticky left-nav rail with live alert counters for quick section jumping (`All`, `Server`, `Pools`, `Datasets`, `VMs`, `Stacks`, `Containers`), collapsing smoothly on mobile.
- **Slide-In Detail Drawers**: Deep dive into hardware, vdevs, storage mountpoints, network interfaces, and container mounts without leaving the page. Fully keyboard accessible (`Escape` to close, `Tab` focus trap, focus restoration).
- **Global Search Filter**: Fast search box (`/` shortcut) filtering pools, datasets, stacks, and virtual machines simultaneously.

---

## Zero-Mutation Read-Only Architecture

**NasWarden** is strictly a read-only telemetry dashboard. It cannot mutate the host, start/stop containers, or alter datasets:
- **TrueNAS API**: Connects over WebSocket (`wss://`) using a scoped API key, performing only read queries (`pool.query`, `pool.dataset.query`, `vm.query`, `system.info`).
- **Docker Isolation**: Never mounts `/var/run/docker.sock` directly into NasWarden. It communicates over HTTP with a read-only [`tecnativa/docker-socket-proxy`](https://github.com/Tecnativa/docker-socket-proxy) container scoped strictly to `CONTAINERS=1` (all mutating POST, PUT, and DELETE calls are blocked at the network proxy).
- **Incus API**: Communicates via mTLS REST API (`GET /1.0/instances?recursion=2`).

---

## Prometheus Metrics & PromQL Alerts

NasWarden exposes Prometheus gauges at `/metrics` for integration with Prometheus, Alertmanager, or Grafana:

| Metric | Type | Description |
|---|---|---|
| `naswarden_dataset_quota_used_bytes{dataset}` | Gauge | Bytes used on a ZFS dataset with a quota |
| `naswarden_dataset_quota_bytes{dataset}` | Gauge | Configured quota/refquota limit in bytes |
| `naswarden_pool_allocated_bytes{pool}` | Gauge | Allocated storage space in bytes |
| `naswarden_pool_size_bytes{pool}` | Gauge | Total storage pool capacity in bytes |
| `naswarden_pool_healthy{pool}` | Gauge | `1` if pool is ONLINE and healthy, `0` if degraded/faulted |
| `naswarden_instance_up{instance, manager, type}` | Gauge | `1` if VM or LXC container is running, `0` otherwise |
| `naswarden_container_up{container, stack}` | Gauge | `1` if Docker container is running, `0` otherwise |
| `naswarden_truenas_alert_count{level}` | Gauge | Active, un-dismissed TrueNAS alerts by severity |
| `naswarden_disk_temperature_celsius{pool, disk}` | Gauge | Current drive temperature (active drives only) |
| `naswarden_replication_task_status{task_id, name, target_pool, state, job_state}` | Gauge | `1` for a task's current state |
| `naswarden_replication_last_success_timestamp_seconds{name}` | Gauge | Unix time the task last finished successfully (remembered across failed runs) |
| `naswarden_last_refresh_success_timestamp_seconds` | Gauge | Unix time of the last successful refresh of the core TrueNAS data |
| `naswarden_source_stale{source}` | Gauge | `1` if that source (`docker`, `truenas-vms`, `incus`, `disks`, `alerts`, `replication`) failed its last refresh and its previous data is being served |

### Alerting Rules Examples

#### 1. Predictive Quota Exhaustion (7-Day Forecast)
Using PromQL's `predict_linear()` over raw byte counts to alert before a dataset runs out of space based on the last 4 hours of growth:
```promql
predict_linear(naswarden_dataset_quota_used_bytes[4h], 86400 * 7) > naswarden_dataset_quota_bytes
```

#### 2. High Quota Utilization (> 90%)
```promql
(naswarden_dataset_quota_used_bytes / naswarden_dataset_quota_bytes) >= 0.90
```

#### 3. Storage Pool Degradation
```promql
naswarden_pool_healthy == 0
```

#### 4. Critical Service Down
```promql
naswarden_container_up{stack="media", container="jellyfin"} == 0
```

#### 5. Ready-made rules

`deploy/prometheus/naswarden.rules.yml` contains 10 rules covering NasWarden itself (refresh stale for 3 min, a source not updating for 5 min, not scraped), TrueNAS (critical alerts, unresolved warnings, degraded pool, disk over 55 C / 60 C) and replication (task failed, no success for 26 h). Add it to `rule_files` and adjust thresholds to taste. Validate with `promtool check rules deploy/prometheus/naswarden.rules.yml`.

---

## Configuration

All configuration is provided via environment variables:

| Env Var | Required | Default | Description |
|---|---|---|---|
| `TRUENAS_HOST` | **Yes** | — | TrueNAS host and port, e.g. `192.0.2.10:8443` |
| `TRUENAS_API_KEY` | **Yes** | — | API key generated in TrueNAS (Credentials → API Keys) |
| `TRUENAS_TLS` | No | `false` | Set `true` to connect over `wss://` |
| `TRUENAS_INSECURE_TLS` | No | `false` | Set `true` to skip certificate validation (e.g. self-signed TrueNAS certs) |
| `DOCKER_PROXY_URL` | No | — | URL to read-only Docker socket proxy, e.g. `http://docker-proxy:2375` |
| `INCUS_URL` | No | — | Incus server API URL, e.g. `https://192.0.2.10:8444` |
| `INCUS_CLIENT_CERT` | No | — | Incus client certificate (PEM string, base64, or file path) |
| `INCUS_CLIENT_KEY` | No | — | Incus client private key (PEM string, base64, or file path) |
| `INCUS_INSECURE_TLS` | No | `true` | Set `false` to enforce Incus server TLS certificate validation |
| `PORT` | No | `8080` | HTTP port for the web UI and `/metrics` endpoint |

---

## Deployment

### Docker Compose with Read-Only Socket Proxy

```yaml
services:
  naswarden:
    image: ghcr.io/ahmd-soliman/naswarden:latest
    container_name: naswarden
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - TRUENAS_HOST=192.0.2.10:8443
      - TRUENAS_API_KEY=${TRUENAS_API_KEY}
      - TRUENAS_TLS=true
      - TRUENAS_INSECURE_TLS=true
      - DOCKER_PROXY_URL=http://docker-proxy:2375
      - INCUS_URL=https://192.0.2.10:8444
      - INCUS_CLIENT_CERT=${INCUS_CLIENT_CERT}
      - INCUS_CLIENT_KEY=${INCUS_CLIENT_KEY}
      - INCUS_INSECURE_TLS=true
    depends_on:
      docker-proxy:
        condition: service_healthy

  # Scoped read-only proxy: blocks all write/mutating API requests
  docker-proxy:
    image: tecnativa/docker-socket-proxy:latest
    container_name: naswarden-docker-proxy
    restart: unless-stopped
    environment:
      - CONTAINERS=1 # Allow read access to list, inspect, and stats
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1:2375/version"]
      interval: 10s
      timeout: 5s
      retries: 3
```

### Resilience

- **Reconnects on its own** if the TrueNAS websocket drops (reboot, network blip); no container restart needed.
- **A failing source keeps its last good data.** If Docker, Incus or the TrueNAS VM query fails, its cards and metrics stay at the previous snapshot instead of vanishing, and the header shows `partial` naming the source.
- **`/healthz`** returns `503` once no refresh has succeeded for three intervals (3 minutes), so a container healthcheck notices a lost TrueNAS.
- **`/ws` is same-origin only.** A browser page served from a different host than the one it connects to is refused. Behind a reverse proxy, keep the original `Host` header (Caddy and nginx `proxy_set_header Host $host` do).

---

## Development

```bash
# Run backend
export TRUENAS_HOST="192.0.2.10:8443"
export TRUENAS_API_KEY="your-api-key"
export TRUENAS_TLS=true
export TRUENAS_INSECURE_TLS=true
go run ./cmd/naswarden

# Build frontend
cd web
npm install
npm run build

# Tests (CI runs all of these and checks the committed internal/web/dist is current)
golangci-lint run ./...   # config in .golangci.yml
go test -race ./...
npm run lint
npm test
```
