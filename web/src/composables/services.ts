import type { Pool, Service, Share } from './usePoolSocket'

// Names TrueNAS uses in service.query, as people know them. Anything not listed
// is shown as TrueNAS names it.
const LABELS: Record<string, string> = {
  cifs: 'SMB',
  nfs: 'NFS',
  ssh: 'SSH',
  iscsitarget: 'iSCSI',
  ups: 'UPS',
  ftp: 'FTP',
  snmp: 'SNMP',
  smartd: 'S.M.A.R.T.',
  lldp: 'LLDP',
  webdav: 'WebDAV',
  openvpn_server: 'OpenVPN server',
  openvpn_client: 'OpenVPN client',
}

export const serviceLabel = (name: string) => LABELS[name] ?? name

export type ServiceTone = 'ok' | 'crit' | 'gray'

// Enabled-but-stopped is the problem, running is fine. Anything else is neutral:
// the backend only sends services that are enabled or running, so this is a
// state TrueNAS added that we do not know.
export function serviceTone(s: Service): ServiceTone {
  if (s.problem) return 'crit'
  return s.state === 'RUNNING' ? 'ok' : 'gray'
}

export function serviceStateText(s: Service): string {
  if (s.problem) return 'Stopped, but set to start on boot'
  if (s.state === 'RUNNING') return s.enable ? 'Running' : 'Running (not set to start on boot)'
  return s.state ? s.state.charAt(0) + s.state.slice(1).toLowerCase() : 'Unknown'
}

export const sharesOf = (shares: Share[], service: string) => shares.filter((sh) => sh.service === service)

// Why a share cannot work, or '' when nothing is wrong. Only enabled shares are
// judged: a disabled one is not served on purpose. A locked dataset cannot be
// served either. Paths under /mnt/<pool>/ need
// that pool to exist and not be offline; other paths cannot be checked here.
export function shareIssue(share: Share, pools: Pool[]): string {
  if (!share.enabled) return ''
  if (!share.path) return 'No path set'
  if (share.locked) return 'Dataset is locked'
  const m = /^\/mnt\/([^/]+)/.exec(share.path)
  if (!m) return ''
  const pool = pools.find((p) => p.name === m[1])
  if (!pool) return `Pool ${m[1]} is not available`
  if (['OFFLINE', 'UNAVAIL', 'FAULTED', 'REMOVED'].includes(pool.status)) return `Pool ${m[1]} is ${pool.status.toLowerCase()}`
  return ''
}

// What the Server rail item's badge counts: services that should be running and
// are not, plus enabled shares that cannot work. 'crit' when a service is down,
// 'warn' when only shares are affected.
export function serviceAttention(services: Service[], shares: Share[], pools: Pool[]): { count: number; level: 'crit' | 'warn' } {
  const down = services.filter((s) => s.problem).length
  const broken = shares.filter((sh) => shareIssue(sh, pools) !== '').length
  return { count: down + broken, level: down > 0 ? 'crit' : 'warn' }
}
