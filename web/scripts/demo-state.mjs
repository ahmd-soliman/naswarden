// Demo state for the README screenshots: a made-up TrueNAS with catalog apps,
// in the shape of the "state" message the backend sends over /ws. Nothing here
// comes from a real system: names, addresses and serials are invented.

const TB = 1e12
const GB = 1e9
const MB = 1e6
const GiB = 2 ** 30

function child(disk, temp, standby = false) {
  return { disk, status: 'ONLINE', read_errors: 0, write_errors: 0, checksum_errors: 0, temperature_c: standby ? null : temp, standby }
}

// usable: what the root dataset reports, as a share of the raw zpool figures
// (RAIDZ parity and ZFS's slop reservation are gone from it).
function pool(name, size, allocated, type, vdev, children, usable = 0.97) {
  const usableSize = Math.round(size * usable)
  const usableUsed = Math.round(allocated * usable)
  return {
    name,
    status: 'ONLINE',
    healthy: true,
    warning: false,
    size,
    allocated,
    free: size - allocated,
    usable_size: usableSize,
    usable_used: usableUsed,
    usable_available: usableSize - usableUsed,
    fragmentation: '8',
    scan: { function: 'SCRUB', state: 'FINISHED', errors: 0 },
    vdevs: [{ name: vdev, type, status: 'ONLINE', children }],
  }
}

function disk(name, model, size, type, pool, temp, read, write, { rpm, standby = false, bus = 'ATA' } = {}) {
  const d = {
    name,
    model,
    serial: `DEMO${name.toUpperCase()}`,
    size,
    type,
    bus,
    pool,
    status: 'ONLINE',
    standby,
    read_errors: 0,
    write_errors: 0,
    checksum_errors: 0,
    read_bytes: 1.4 * TB,
    write_bytes: 3.1 * TB,
    read_bytes_per_sec: read,
    write_bytes_per_sec: write,
  }
  if (rpm) d.rotation_rate = rpm
  if (!standby) Object.assign(d, { temp_c: temp, temp_7d_min: temp - 4, temp_7d_avg: temp - 0.5, temp_7d_max: temp + 5 })
  return d
}

function dataset(name, used, quota) {
  return {
    name,
    used,
    quota,
    quota_source: 'refquota',
    mountpoint: `/mnt/${name}`,
    compression: 'LZ4',
    compress_ratio: '1.12x',
    recordsize: '128K',
    encrypted: false,
    used_by_snapshots: Math.round(used / 12),
  }
}

// Catalog app versions, for the rendered compose file path TrueNAS uses.
const APP_VERSIONS = {
  jellyfin: '1.3.2',
  immich: '1.9.4',
  nextcloud: '2.1.8',
  'home-assistant': '1.6.1',
  'paperless-ngx': '1.4.0',
  syncthing: '1.2.5',
  'uptime-kuma': '1.1.9',
  naswarden: '1.0.2',
}

function container(now, i, app, service, image, cpu, memMB, { state = 'running', port } = {}) {
  const running = state === 'running'
  return {
    name: `ix-${app}-${service}-1`,
    image,
    state,
    status: running ? 'Up 6 days (healthy)' : 'Exited (1) 2 hours ago',
    cpu_percent: running ? cpu : 0,
    mem_used: running ? memMB * MB : 0,
    mem_limit: 8 * GB,
    started_at: new Date((now - 6 * 86400) * 1000).toISOString(),
    restart_policy: 'unless-stopped',
    command: '',
    mounts: [],
    networks: [{ name: `ix-${app}_default`, ip: `172.16.${i + 2}.2` }],
    ports: port ? [`${port}/tcp → ${port}`] : [],
    stack: `ix-${app}`,
    compose_file: `/mnt/.ix-apps/app_configs/${app}/versions/${APP_VERSIONS[app]}/templates/rendered/docker-compose.yaml`,
    icon: '',
    exit_code: running ? 0 : 1,
    oom_killed: false,
    optional: false,
  }
}

function vm(now, name, type, os, status, cpu, memGB, memTotalGB, cores, ip, { manager = 'incus', isVM = true } = {}) {
  const running = status === 'Running'
  return {
    name,
    type,
    manager,
    status,
    status_code: running ? 103 : 102,
    is_vm: isVM,
    os,
    kernel: '',
    arch: 'amd64',
    cpu_cores: cores,
    cpu_percent: running ? cpu : 0,
    mem_used: running ? memGB * GB : 0,
    mem_total: memTotalGB * GB,
    disk_used: 18 * GB,
    disk_total: 64 * GB,
    disk_pool: 'fast',
    ipv4: running ? [ip] : [],
    mac: '',
    bridge: 'br0',
    started_at: new Date((now - 9 * 86400) * 1000).toISOString(),
    auto_start: running,
    config: {},
  }
}

// now: Unix seconds; timestamps in the state are relative to it, so "updated
// 8s ago" and "6h ago" read the same whenever the screenshots are taken.
// A data protection task as the backend sends it, with the plain defaults.
function protectionTask(o) {
  return { exclude: [], enabled: true, overdue: false, ...o }
}

export function demoState(now = Math.floor(Date.now() / 1000)) {
  const apps = [
    ['jellyfin', 'jellyfin', 'jellyfin/jellyfin:10.11.3', 3.4, 912, { port: 30013 }],
    ['immich', 'server', 'ghcr.io/immich-app/immich-server:v2.3.1', 1.9, 1450, { port: 30041 }],
    ['immich', 'machine-learning', 'ghcr.io/immich-app/immich-machine-learning:v2.3.1', 0.4, 1210],
    ['immich', 'pgvecto', 'ghcr.io/immich-app/postgres:15-vectorchord', 0.3, 420],
    ['immich', 'redis', 'valkey/valkey:8', 0.1, 18],
    ['nextcloud', 'nextcloud', 'nextcloud:32.0.2', 0.8, 286, { port: 30027 }],
    ['nextcloud', 'postgres', 'postgres:17.6', 0.2, 96],
    ['nextcloud', 'redis', 'redis:8.2', 0.1, 9],
    ['home-assistant', 'home-assistant', 'ghcr.io/home-assistant/home-assistant:2026.11.2', 1.1, 512, { port: 30103 }],
    ['paperless-ngx', 'paperless', 'ghcr.io/paperless-ngx/paperless-ngx:2.20.1', 0.6, 640, { port: 30070 }],
    ['paperless-ngx', 'redis', 'redis:8.2', 0.1, 7],
    ['syncthing', 'syncthing', 'syncthing/syncthing:2.0.11', 0.3, 64, { port: 30090 }],
    ['uptime-kuma', 'uptime-kuma', 'louislam/uptime-kuma:2.0.2', 0, 0, { state: 'exited', port: 31050 }],
    ['naswarden', 'naswarden', 'ghcr.io/ahmd-soliman/naswarden:1.1.0', 0.2, 30, { port: 30597 }],
    ['ddns-updater', 'ddns-updater', 'qmcgaw/ddns-updater:2.9', 0, 14, { port: 30201 }],
    ['elasticsearch', 'elasticsearch', 'elasticsearch:8.15.0', 1.5, 3100],
    ['gitlab-cloud-runner', 'gitlab-cloud-runner', 'gitlab/gitlab-runner:v18.5', 0, 76],
    ['nextcloud-aio', 'mastercontainer', 'nextcloud/all-in-one:latest', 0.3, 110, { port: 30210 }],
    ['open-speed-test', 'open-speed-test', 'openspeedtest/latest', 0, 18, { port: 30215 }],
  ]

  return {
    type: 'state',
    updated_at: now - 8,
    server: {
      hostname: 'truenas',
      version: '25.10.1',
      uptime_seconds: 9 * 86400 + 4 * 3600,
      cpu_percent: 14,
      mem_used: 38.2 * GiB,
      mem_total: 64 * GiB,
      load_percent_1: 18,
      load_percent_5: 16,
      load_percent_15: 15,
      arc_bytes: 18 * GiB,
      update: { available: true, version: '25.10.2', release_notes_url: 'https://www.truenas.com/docs/', apps_with_updates: 2, checked_at: now - 3600 },
      interfaces: [
        {
          name: 'enp3s0',
          type: 'PHYSICAL',
          speed_mbps: 2500,
          rx_kbps: 9800,
          tx_kbps: 2300,
          link_state: 'LINK_STATE_UP',
          speed: '2500Mb/s Twisted Pair',
          addresses: ['192.168.1.10/24'],
        },
      ],
      net_rx_kbps: 9800,
      net_tx_kbps: 2300,
      cpu_model: 'Intel Core i5-12400',
      cores: 12,
      physical_cores: 6,
      cpu_temp_c: 47,
    },
    pools: [
      pool('tank', 43 * TB, 21 * TB, 'RAIDZ1', 'raidz1-0', [child('sda', 41), child('sdb', 42), child('sdc', 43), child('sdd', 41)], 0.73),
      pool('fast', 3.6 * TB, 1.1 * TB, 'MIRROR', 'mirror-0', [child('sde', 36), child('sdf', 37)]),
      pool('backup', 12 * TB, 4.4 * TB, 'MIRROR', 'mirror-0', [child('sdg', 0, true), child('sdh', 0, true)]),
      pool('boot-pool', 238 * GB, 6 * GB, 'STRIPE', 'stripe', [child('nvme0n1', 39)]),
    ],
    disks: [
      disk('sda', 'WDC WD120EFBX', 12 * TB, 'HDD', 'tank', 41, 2.1 * MB, 640e3, { rpm: 7200 }),
      disk('sdb', 'WDC WD120EFBX', 12 * TB, 'HDD', 'tank', 42, 2.0 * MB, 655e3, { rpm: 7200 }),
      disk('sdc', 'WDC WD120EFBX', 12 * TB, 'HDD', 'tank', 43, 2.2 * MB, 630e3, { rpm: 7200 }),
      disk('sdd', 'WDC WD120EFBX', 12 * TB, 'HDD', 'tank', 41, 2.1 * MB, 648e3, { rpm: 7200 }),
      disk('sde', 'Samsung SSD 870 EVO 4TB', 4 * TB, 'SSD', 'fast', 36, 5.8 * MB, 1.9 * MB),
      disk('sdf', 'Samsung SSD 870 EVO 4TB', 4 * TB, 'SSD', 'fast', 37, 5.7 * MB, 1.9 * MB),
      disk('sdg', 'ST12000VN0008', 12 * TB, 'HDD', 'backup', 0, 0, 0, { rpm: 7200, standby: true }),
      disk('sdh', 'ST12000VN0008', 12 * TB, 'HDD', 'backup', 0, 0, 0, { rpm: 7200, standby: true }),
      disk('nvme0n1', 'WD Blue SN580 250GB', 250 * GB, 'SSD', 'boot-pool', 39, 12e3, 85e3, { bus: 'NVME' }),
    ],
    datasets: [
      dataset('tank/media', 13.4 * TB, 16 * TB),
      dataset('tank/photos', 2.9 * TB, 4 * TB),
      dataset('tank/backups', 3.3 * TB, 6 * TB),
      dataset('fast/apps', 312 * GB, 800 * GB),
      dataset('tank/documents', 214 * GB, 1 * TB),
      dataset('tank/TimeMachine/mac2', 1.7 * TB, 3 * TB),
      dataset('backup/tank/app-config/incus/incus-gitlab-runner/registry-cache', 891.9 * MB, 50 * GB),
    ],
    containers: apps.map((a, i) => container(now, i, ...a)),
    vms: [
      vm(now, 'ubuntu-dev', 'virtual-machine', 'Ubuntu 24.04', 'Running', 6.2, 3.1, 8, 4, '192.168.1.31'),
      vm(now, 'pihole', 'container', 'Debian 13', 'Running', 0.4, 0.2, 1, 1, '192.168.1.32', { isVM: false }),
      vm(now, 'k8s-node-0', 'virtual-machine', 'Ubuntu 24.04', 'Running', 4.1, 5.2, 8, 4, '192.168.1.141'),
      vm(now, 'incus-gitlab-runner', 'container', 'Debian 13', 'Running', 3.2, 1.3, 32, 12, '192.168.1.125', { isVM: false }),
      vm(now, 'windows-11', 'virtual-machine', 'Windows 11', 'Stopped', 0, 0, 16, 8, '', { manager: 'truenas' }),
    ],
    alerts: [
      {
        id: 'demo-1',
        uuid: 'demo-1',
        level: 'WARNING',
        source: 'Quota',
        klass: 'QuotaWarning',
        formatted: 'Quota exceeded on dataset tank/media. Used 83.75% (13.4 TB of 16 TB).',
        text: '',
        datetime: now - 5 * 3600,
        dismissed: false,
      },
    ],
    replications: [
      {
        id: 1,
        name: 'tank-backup',
        direction: 'PUSH',
        transport: 'LOCAL',
        source_datasets: ['tank/photos', 'tank/documents'],
        target_dataset: 'backup/tank',
        target_pool: 'backup',
        source_pools: ['tank'],
        state: 'FINISHED',
        job_state: 'SUCCESS',
        progress_percent: 100,
        progress_description: '',
        last_snapshot: 'tank/photos@auto-daily',
        time_started: now - 6 * 3600 - 420,
        time_finished: now - 6 * 3600,
        duration_seconds: 420,
        enabled: true,
      },
    ],
    services: [
      { name: 'cifs', state: 'RUNNING', enable: true, problem: false },
      { name: 'nfs', state: 'STOPPED', enable: true, problem: true },
      { name: 'ssh', state: 'RUNNING', enable: true, problem: false },
      { name: 'smartd', state: 'RUNNING', enable: true, problem: false },
    ],
    shares: [
      { service: 'cifs', name: 'media', path: '/mnt/tank/media', enabled: true, read_only: true },
      { service: 'cifs', name: 'documents', path: '/mnt/tank/documents', enabled: true, read_only: false },
      { service: 'cifs', name: 'scratch', path: '/mnt/gone/scratch', enabled: true, read_only: false },
      { service: 'cifs', name: 'vault', path: '/mnt/tank/vault', enabled: true, read_only: false, locked: true },
      { service: 'nfs', name: '', path: '/mnt/fast/apps', enabled: true, read_only: false },
    ],
    protection: [
      protectionTask({ kind: 'snapshot', id: 1, name: 'tank/media', source: 'tank/media', schedule: '0 0 * * *', retention: '2 WEEK', recursive: true, state: 'OK', time_finished: now - 15 * 3600 }),
      protectionTask({ kind: 'snapshot', id: 2, name: 'tank/photos', source: 'tank/photos', schedule: '0 0 * * *', retention: '2 WEEK', recursive: true, state: 'OK', time_finished: now - 15 * 3600 }),
      protectionTask({ kind: 'snapshot', id: 3, name: 'tank/documents', source: 'tank/documents', schedule: '0 0 * * *', retention: '2 WEEK', recursive: true, state: 'OK', time_finished: now - 15 * 3600 }),
      protectionTask({ kind: 'snapshot', id: 4, name: 'fast/apps', source: 'fast/apps', schedule: '0 0 * * *', retention: '2 WEEK', recursive: true, state: 'OK', time_finished: now - 15 * 3600 }),
      protectionTask({ kind: 'snapshot', id: 5, name: 'tank/backups', source: 'tank/backups', schedule: '0 0 * * *', retention: '2 WEEK', recursive: true, state: 'OK', time_finished: now - 15 * 3600 }),
      protectionTask({ kind: 'cloudsync', id: 1, name: 'Photos to cloud', source: '/mnt/tank/photos', remote: 'S3 photos-archive', direction: 'PUSH', schedule: '0 2 * * *', state: 'OK', time_started: now - 13 * 3600, time_finished: now - 13 * 3600 + 540, duration_seconds: 540 }),
      protectionTask({ kind: 'cloudsync', id: 2, name: 'Documents mirror', source: '/mnt/tank/documents', remote: 'GOOGLE_DRIVE mirror', direction: 'PUSH', schedule: '30 3 * * *', state: 'FAILED', error: "[EFAULT] Cloud Sync failed: couldn't login to Google Drive: invalid_grant: Token has been expired or revoked.", time_started: now - 12 * 3600, time_finished: now - 12 * 3600 + 25, duration_seconds: 25 }),
      protectionTask({ kind: 'cloudsync', id: 3, name: 'Archive upload', source: '/mnt/tank/backups', remote: 'B2 cold-archive', direction: 'PUSH', schedule: '0 4 * * 0', state: 'RUNNING', progress_percent: 41.5, progress_description: '212 / 511 files, 38.2 GiB / 92 GiB', time_started: now - 50 * 60 }),
      protectionTask({ kind: 'rsync', id: 1, name: 'Offsite copy', source: '/mnt/tank/documents', remote: 'backup-host:/srv/in', direction: 'PUSH', schedule: '0 1 * * mon,thu', state: 'OK', overdue: true, time_started: now - 6 * 86400, time_finished: now - 6 * 86400 + 300, duration_seconds: 300 }),
      protectionTask({ kind: 'cloud_backup', id: 1, name: 'Restic to S3', source: '/mnt/tank/photos', remote: 'S3 restic', schedule: '0 5 * * *', retention: 'last 14', state: 'NEVER' }),
      protectionTask({ kind: 'cloud_backup', id: 2, name: 'Old backup', source: '/mnt/tank/old', remote: 'S3 old', schedule: '0 5 * * *', enabled: false, state: 'DISABLED', time_started: now - 40 * 86400, time_finished: now - 40 * 86400 + 60 }),
    ],
    stale_sources: [],
    docker: { state: 'on', access: 'socket' },
  }
}
