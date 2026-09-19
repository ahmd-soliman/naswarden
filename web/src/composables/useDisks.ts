import type { Disk } from './usePoolSocket'

export type Tone = 'ok' | 'warn' | 'crit' | 'gray'

// Same thresholds as the shipped Prometheus rules (DiskHot / DiskVeryHot).
export const TEMP_WARN = 55
export const TEMP_CRIT = 60

export const tempTone = (c: number): Tone => (c >= TEMP_CRIT ? 'crit' : c >= TEMP_WARN ? 'warn' : 'ok')

export const zfsErrors = (d: Disk) => d.read_errors + d.write_errors + d.checksum_errors

const toneClass: Record<Tone, string> = { ok: 'badge--green', warn: 'badge--yellow', crit: 'badge--red', gray: 'badge--gray' }

// One status per disk. A pool member that is not ONLINE, or has ZFS errors,
// needs attention; a sleeping drive is neutral, not a problem.
export function diskBadge(d: Disk): { cls: string; label: string; tone: Tone } {
  const mk = (tone: Tone, label: string) => ({ cls: toneClass[tone], label, tone })
  if (d.status && d.status !== 'ONLINE') return mk('crit', d.status.toLowerCase())
  if (zfsErrors(d) > 0) return mk('warn', 'errors')
  if (d.standby) return mk('gray', 'standby')
  return mk('ok', 'healthy')
}

// Does this disk deserve a badge on the rail?
export const diskNeedsAttention = (d: Disk) => {
  const t = diskBadge(d).tone
  return t === 'crit' || t === 'warn' || (d.temp_c !== undefined && d.temp_c >= TEMP_WARN)
}

export const diskTypeLabel = (d: Disk) =>
  d.type === 'HDD' && d.rotation_rate ? `HDD ${d.rotation_rate}` : d.type

// Text for the temperature range bar (screen readers and tooltip).
export function tempRangeLabel(d: Disk): string {
  if (d.temp_c === undefined) return d.standby ? 'Standby, no temperature reading' : 'No temperature reading'
  const range =
    d.temp_7d_min !== undefined && d.temp_7d_max !== undefined
      ? `, 7-day range ${Math.round(d.temp_7d_min)} to ${Math.round(d.temp_7d_max)} °C`
      : ''
  return `${Math.round(d.temp_c)} °C${range}`
}

export const totalRates = (disks: Disk[]) =>
  disks.reduce(
    (acc, d) => ({
      read: acc.read + (d.read_bytes_per_sec ?? 0),
      write: acc.write + (d.write_bytes_per_sec ?? 0),
      known: acc.known || d.read_bytes_per_sec !== undefined,
    }),
    { read: 0, write: 0, known: false },
  )
