import { describe, expect, it } from 'vitest'
import type { Disk } from './usePoolSocket'
import { diskBadge, diskNeedsAttention, diskTypeLabel, tempRangeLabel, tempTone, totalRates, zfsErrors } from './useDisks'
import { formatBitrate, formatCapacity, formatRate } from './format'
import { matchesDisk } from './search'

const disk = (o: Partial<Disk> = {}): Disk => ({
  name: 'sda', model: 'ST24000NM000C-3WD103', serial: 'SN000001', size: 24e12, type: 'HDD', rotation_rate: 7200,
  pool: 'tank', status: 'ONLINE', standby: false, temp_c: 49, read_errors: 0, write_errors: 0, checksum_errors: 0, ...o,
})

describe('tempTone', () => {
  it('matches the alert thresholds', () => {
    expect(tempTone(54.9)).toBe('ok')
    expect(tempTone(55)).toBe('warn')
    expect(tempTone(59.9)).toBe('warn')
    expect(tempTone(60)).toBe('crit')
  })
})

describe('diskBadge', () => {
  it('is healthy by default', () => expect(diskBadge(disk())).toMatchObject({ label: 'healthy', tone: 'ok' }))
  it('flags a pool member that is not ONLINE', () =>
    expect(diskBadge(disk({ status: 'DEGRADED' }))).toMatchObject({ label: 'degraded', tone: 'crit' }))
  it('flags ZFS errors', () => {
    const d = disk({ checksum_errors: 2 })
    expect(zfsErrors(d)).toBe(2)
    expect(diskBadge(d)).toMatchObject({ label: 'errors', tone: 'warn' })
  })
  it('treats standby as neutral, not a problem', () => {
    const d = disk({ standby: true, temp_c: undefined })
    expect(diskBadge(d)).toMatchObject({ label: 'standby', tone: 'gray' })
    expect(diskNeedsAttention(d)).toBe(false)
  })
  it('a boot disk (no pool status) is healthy', () => expect(diskBadge(disk({ status: '', pool: 'boot-pool' })).tone).toBe('ok'))
  it('needs attention when hot even if healthy', () => expect(diskNeedsAttention(disk({ temp_c: 56 }))).toBe(true))
})

describe('labels', () => {
  it('type label carries rpm for spinning disks only', () => {
    expect(diskTypeLabel(disk())).toBe('HDD 7200')
    expect(diskTypeLabel(disk({ type: 'SSD', rotation_rate: undefined }))).toBe('SSD')
    expect(diskTypeLabel(disk({ rotation_rate: undefined }))).toBe('HDD')
  })
  it('range label for screen readers', () => {
    expect(tempRangeLabel(disk({ temp_7d_min: 47, temp_7d_max: 55 }))).toBe('49 °C, 7-day range 47 to 55 °C')
    expect(tempRangeLabel(disk({ temp_c: undefined, standby: true }))).toBe('Standby, no temperature reading')
  })
  it('sums throughput only when a rate is known', () => {
    expect(totalRates([disk()]).known).toBe(false)
    expect(totalRates([disk({ read_bytes_per_sec: 100, write_bytes_per_sec: 300 }), disk({ read_bytes_per_sec: 50, write_bytes_per_sec: 0 })])).toEqual({ read: 150, write: 300, known: true })
  })
})

describe('formatting', () => {
  it('formats disk rates', () => {
    expect(formatRate(0)).toBe('0 B/s')
    expect(formatRate(1_800_000)).toBe('1.7 MB/s')
  })
  it('formats network rates from kilobits/s', () => {
    expect(formatBitrate(640)).toBe('640 kb/s')
    expect(formatBitrate(2000)).toBe('2.0 Mb/s')
    expect(formatBitrate(1_250_000)).toBe('1.3 Gb/s')
  })
})

describe('formatCapacity', () => {
  it('uses decimal units like the drive label', () => {
    expect(formatCapacity(24_000_277_250_048)).toBe('24.0 TB')
    expect(formatCapacity(250_059_350_016)).toBe('250.1 GB')
    expect(formatCapacity(999)).toBe('999 B')
  })
})

describe('matchesDisk', () => {
  it('matches name, model, serial, pool and type', () => {
    for (const q of ['sda', 'st24000', 'sn0000', 'tank', 'hdd']) expect(matchesDisk(disk(), q)).toBe(true)
    expect(matchesDisk(disk(), 'samsung')).toBe(false)
    expect(matchesDisk(disk(), '')).toBe(true)
  })
})
