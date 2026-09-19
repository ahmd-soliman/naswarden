import { describe, expect, it } from 'vitest'
import { formatBitrate, formatBytes, formatCapacity, formatRate } from './format'

describe('format', () => {
  it('formatBytes uses binary units and one decimal', () => {
    expect(formatBytes(0)).toBe('0.0 B')
    expect(formatBytes(1023)).toBe('1023.0 B')
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(1024 ** 3 * 1.5)).toBe('1.5 GB')
    expect(formatBytes(1024 ** 6)).toBe('1048576.0 TB') // stops at TB rather than inventing units
  })
  it('formatRate shows whole bytes below 1 KB and units above', () => {
    expect(formatRate(0)).toBe('0 B/s')
    expect(formatRate(511.6)).toBe('512 B/s')
    expect(formatRate(1024 * 1024 * 1.8)).toBe('1.8 MB/s')
  })
  it('formatBitrate switches at the SI thresholds (input is kilobits/s)', () => {
    expect(formatBitrate(0)).toBe('0 kb/s')
    expect(formatBitrate(999)).toBe('999 kb/s')
    expect(formatBitrate(1000)).toBe('1.0 Mb/s')
    expect(formatBitrate(940_000)).toBe('940.0 Mb/s')
    expect(formatBitrate(1_000_000)).toBe('1.0 Gb/s')
  })
  it('formatCapacity is decimal, like the label on the drive', () => {
    expect(formatCapacity(24e12)).toBe('24.0 TB')
    expect(formatCapacity(500e9)).toBe('500.0 GB')
    expect(formatCapacity(512)).toBe('512 B')
    expect(formatCapacity(2e15)).toBe('2.0 PB')
  })
})
