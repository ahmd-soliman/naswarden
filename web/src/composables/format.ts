export function formatBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`
}

// Disk throughput, e.g. 1.8 MB/s (bytes per second, binary units like the rest of the UI).
export function formatRate(bytesPerSec: number): string {
  if (bytesPerSec < 1024) return `${Math.round(bytesPerSec)} B/s`
  return `${formatBytes(bytesPerSec)}/s`
}

// Network rate. netdata reports kilobits per second; show bits with SI prefixes.
export function formatBitrate(kbps: number): string {
  if (kbps >= 1_000_000) return `${(kbps / 1_000_000).toFixed(1)} Gb/s`
  if (kbps >= 1000) return `${(kbps / 1000).toFixed(1)} Mb/s`
  return `${kbps.toFixed(0)} kb/s`
}

// Drive capacity the way it is printed on the label (decimal: 24.0 TB, not 21.8 TiB).
export function formatCapacity(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let value = bytes
  let unitIndex = 0
  while (value >= 1000 && unitIndex < units.length - 1) {
    value /= 1000
    unitIndex++
  }
  return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}

// A dataset or path split at its last "/": the part you recognise (leaf) and the
// path above it (parent, with its trailing slash, so parent + leaf is the name).
export function splitPath(name: string): { parent: string; leaf: string } {
  const i = name.lastIndexOf('/')
  if (i < 0 || i === name.length - 1) return { parent: '', leaf: name }
  return { parent: name.slice(0, i + 1), leaf: name.slice(i + 1) }
}
