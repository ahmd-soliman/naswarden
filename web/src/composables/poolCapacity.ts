import type { Pool } from './usePoolSocket'

export interface PoolCapacity {
  used: number
  total: number
  /** Percent of the total in use, 0-100, whole number. */
  percent: number
  /** True when these are the usable figures (the root dataset's), false when they are the zpool's raw ones. */
  usable: boolean
}

/**
 * What the pool card shows: the space files can actually use, from the pool's
 * root dataset. The zpool's raw size counts RAIDZ parity and ZFS's slop
 * reservation, so it overstates the room left. When the backend could not read
 * the root dataset it sends no usable figures, and the raw ones are used
 * instead of showing 0.
 */
export function poolCapacity(pool: Pick<Pool, 'size' | 'allocated' | 'usable_size' | 'usable_used'>): PoolCapacity {
  const usable = (pool.usable_size ?? 0) > 0
  const total = usable ? (pool.usable_size as number) : pool.size
  const used = usable ? (pool.usable_used ?? 0) : pool.allocated
  return { used, total, percent: total > 0 ? Math.round((used / total) * 100) : 0, usable }
}
